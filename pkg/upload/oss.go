package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/google/uuid"
	pkgErrors "github.com/pkg/errors"
)

type OSSConfig struct {
	Endpoint        string
	AccessKeyID     string
	AccessKeySecret string
	Bucket          string
	Region          string
	UseHTTPS        bool
	CDNURL          string
}

type OSSStorage struct {
	client *oss.Client
	bucket *oss.Bucket
	config OSSConfig
}

func NewOSSStorage(cfg OSSConfig) (*OSSStorage, error) {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		return nil, pkgErrors.Wrap(err, "new oss client")
	}
	bucket, err := client.Bucket(cfg.Bucket)
	if err != nil {
		return nil, pkgErrors.Wrap(err, "get bucket")
	}
	return &OSSStorage{
		client: client,
		bucket: bucket,
		config: cfg,
	}, nil
}

func (o *OSSStorage) Type() StorageType { return OSS }

func (o *OSSStorage) Put(ctx context.Context, file *FileInfo, reader io.Reader) error {
	opts := []oss.Option{
		oss.ContentType(file.MimeType),
	}
	err := o.bucket.PutObject(file.Path, reader, opts...)
	if err != nil {
		return pkgErrors.Wrap(err, "put object")
	}
	file.CreatedAt = time.Now()
	url, _ := o.GetURL(ctx, file.Path, file.IsPublic)
	file.URL = url
	return nil
}

func (o *OSSStorage) PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error {
	src, err := header.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	return o.Put(ctx, file, src)
}

func (o *OSSStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return o.bucket.GetObject(path)
}

func (o *OSSStorage) GetURL(ctx context.Context, path string, public bool) (string, error) {
	if public {
		protocol := "http"
		if o.config.UseHTTPS {
			protocol = "https"
		}
		domain := o.config.Bucket + "." + o.config.Endpoint
		if o.config.CDNURL != "" {
			domain = o.config.CDNURL
		}
		return protocol + "://" + domain + "/" + strings.TrimLeft(path, "/"), nil
	}
	// 私有签名URL
	return o.bucket.SignURL(path, oss.HTTPGet, 3600)
}

func (o *OSSStorage) Delete(ctx context.Context, path string) error {
	return o.bucket.DeleteObject(path)
}

func (o *OSSStorage) Stat(ctx context.Context, path string) (*FileInfo, error) {
	props, err := o.bucket.GetObjectMeta(path)
	if err != nil {
		return nil, pkgErrors.Wrap(err, "get object meta")
	}
	// 解析元数据（示例，可根据需要完善）
	size := int64(0)
	if contentLength, ok := props["Content-Length"]; ok && len(contentLength) > 0 {
		fmt.Sscanf(contentLength[0], "%d", &size)
	}
	return &FileInfo{
		ID:        uuid.New().String(),
		Name:      path,
		Path:      path,
		Size:      size,
		CreatedAt: time.Now(),
	}, nil
}

func (o *OSSStorage) Exists(ctx context.Context, path string) (bool, error) {
	return o.bucket.IsObjectExist(path)
}

func (o *OSSStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	_, err := o.bucket.CopyObject(srcPath, dstPath)
	return err
}

func (o *OSSStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	if err := o.Copy(ctx, srcPath, dstPath); err != nil {
		return err
	}
	return o.Delete(ctx, srcPath)
}
