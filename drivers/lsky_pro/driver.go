package lsky_pro

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenListTeam/OpenList/v4/drivers/base"
	"github.com/OpenListTeam/OpenList/v4/internal/driver"
	"github.com/OpenListTeam/OpenList/v4/internal/errs"
	"github.com/OpenListTeam/OpenList/v4/internal/model"
	"github.com/OpenListTeam/OpenList/v4/internal/op"
	"github.com/go-resty/resty/v2"
	log "github.com/sirupsen/logrus"
)

// LskyPro maps the images of a Lsky Pro (v2, API v1) account to a flat list
// of files in the root folder.
type LskyPro struct {
	model.Storage
	Addition
	client *resty.Client
}

func (d *LskyPro) Config() driver.Config          { return config }
func (d *LskyPro) GetAddition() driver.Additional { return &d.Addition }

func (d *LskyPro) Init(ctx context.Context) error {
	d.Address = strings.TrimRight(d.Address, "/")
	d.client = base.NewRestyClient().
		SetBaseURL(d.Address).
		SetHeader("Accept", "application/json")

	if d.Token == "" {
		if d.Email == "" || d.Password == "" {
			return fmt.Errorf("token, or email and password, is required")
		}
		var data TokenData
		err := d.request(ctx, http.MethodPost, "/tokens", func(req *resty.Request) {
			req.SetFormData(map[string]string{"email": d.Email, "password": d.Password})
		}, &data)
		if err != nil {
			return err
		}
		d.Token = data.Token
		op.MustSaveDriverStorage(d)
	}
	d.client.SetAuthToken(strings.TrimPrefix(d.Token, "Bearer "))

	// connectivity check
	return d.request(ctx, http.MethodGet, "/profile", nil, nil)
}

// GetRoot returns the single, flat root folder.
func (d *LskyPro) GetRoot(ctx context.Context) (model.Obj, error) {
	return &model.Object{Name: "root", IsFolder: true, Path: "/"}, nil
}

func (d *LskyPro) Drop(ctx context.Context) error { return nil }

func (d *LskyPro) List(ctx context.Context, dir model.Obj, args model.ListArgs) ([]model.Obj, error) {
	images, err := listAll[Image](ctx, d, "/images", map[string]string{"order": "earliest"})
	if err != nil {
		return nil, err
	}
	objs := make([]model.Obj, 0, len(images))
	for _, i := range images {
		objs = append(objs, i.toObj())
	}
	uniqueNames(objs)
	return objs, nil
}

func (d *LskyPro) Link(ctx context.Context, file model.Obj, args model.LinkArgs) (*model.Link, error) {
	if img, ok := file.(*imageObj); ok && img.url != "" {
		return &model.Link{URL: img.url}, nil
	}
	return nil, errs.NotSupport
}

func (d *LskyPro) Remove(ctx context.Context, obj model.Obj) error {
	return d.request(ctx, http.MethodDelete, "/images/"+obj.GetID(), nil, nil)
}

func (d *LskyPro) Put(ctx context.Context, dstDir model.Obj, file model.FileStreamer, up driver.UpdateProgress) (model.Obj, error) {
	fields := map[string]string{}
	if d.StrategyID != "" {
		fields["strategy_id"] = d.StrategyID
	}
	var img Image
	err := d.request(ctx, http.MethodPost, "/upload", func(req *resty.Request) {
		req.SetMultipartFormData(fields).SetFileReader("file", file.GetName(),
			driver.NewLimitedUploadStream(ctx, &driver.ReaderUpdatingProgress{
				Reader:         file,
				UpdateProgress: up,
			}))
	}, &img)
	if err != nil {
		return nil, err
	}
	if img.OriginName == "" {
		img.OriginName = file.GetName()
	}
	// Lsky Pro keeps same-named images side by side, so replace the old one
	// when OpenList is overwriting an existing file.
	if old := file.GetExist(); old != nil && old.GetID() != img.Key {
		if err := d.Remove(ctx, old); err != nil {
			// keep a single record per name: drop the new upload and fail
			if rerr := d.Remove(ctx, img.toObj()); rerr != nil {
				log.Warnf("lsky_pro: failed to clean up image %s: %v", img.Key, rerr)
			}
			return nil, fmt.Errorf("lsky pro: failed to replace the existing image: %w", err)
		}
	}
	return img.toObj(), nil
}

var (
	_ driver.Driver    = (*LskyPro)(nil)
	_ driver.PutResult = (*LskyPro)(nil)
	_ driver.Remove    = (*LskyPro)(nil)
)
