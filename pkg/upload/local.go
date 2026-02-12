package upload

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	pkgErrors "github.com/pkg/errors"
)

type LocalConfig struct {
	BasePath string
	BaseURL  string
	CDNURL   string
}

type LocalStorage struct {
	config LocalConfig
}

func NewLocalStorage(cfg LocalConfig) (*LocalStorage, error) {
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, pkgErrors.Wrap(err, "create base dir")
	}
	return &LocalStorage{config: cfg}, nil
}

func (l *LocalStorage) Type() StorageType { return Local }

func (l *LocalStorage) fullPath(path string) string {
	return filepath.Join(l.config.BasePath, path)
}

func (l *LocalStorage) Put(ctx context.Context, file *FileInfo, reader io.Reader) error {
	full := l.fullPath(file.Path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return pkgErrors.Wrap(err, "mkdir")
	}
	f, err := os.Create(full)
	if err != nil {
		return pkgErrors.Wrap(err, "create file")
	}
	defer f.Close()

	h := md5.New()
	w := io.MultiWriter(f, h)
	written, err := io.Copy(w, reader)
	if err != nil {
		os.Remove(full)
		return pkgErrors.Wrap(err, "copy")
	}
	file.Size = written
	file.MD5 = hex.EncodeToString(h.Sum(nil))
	file.CreatedAt = time.Now()
	url, _ := l.GetURL(ctx, file.Path, file.IsPublic)
	file.URL = url
	return nil
}

func (l *LocalStorage) PutMultipart(ctx context.Context, file *FileInfo, header *multipart.FileHeader) error {
	src, err := header.Open()
	if err != nil {
		return pkgErrors.Wrap(err, "open multipart")
	}
	defer src.Close()
	return l.Put(ctx, file, src)
}

func (l *LocalStorage) Get(ctx context.Context, path string) (io.ReadCloser, error) {
	return os.Open(l.fullPath(path))
}

func (l *LocalStorage) GetURL(ctx context.Context, path string, public bool) (string, error) {
	base := l.config.BaseURL
	if l.config.CDNURL != "" {
		base = l.config.CDNURL
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/"), nil
}

func (l *LocalStorage) Delete(ctx context.Context, path string) error {
	return os.Remove(l.fullPath(path))
}

func (l *LocalStorage) Stat(ctx context.Context, path string) (*FileInfo, error) {
	info, err := os.Stat(l.fullPath(path))
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		ID:        uuid.New().String(),
		Name:      filepath.Base(path),
		Path:      path,
		Size:      info.Size(),
		CreatedAt: info.ModTime(),
	}, nil
}

func (l *LocalStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(l.fullPath(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (l *LocalStorage) Copy(ctx context.Context, srcPath, dstPath string) error {
	src := l.fullPath(srcPath)
	dst := l.fullPath(dstPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return copyFile(src, dst)
}

func (l *LocalStorage) Move(ctx context.Context, srcPath, dstPath string) error {
	if err := l.Copy(ctx, srcPath, dstPath); err != nil {
		return err
	}
	return l.Delete(ctx, srcPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
