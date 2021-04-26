package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func Upload(ctx context.Context, s3Client *s3.Client, bucket string, key string, src string) ([]string, error) {
	u := manager.NewUploader(s3Client)
	var keys []string

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			rel, err = filepath.Rel(cwd, path)
			if err != nil {
				return err
			}
		}

		k := filepath.ToSlash(filepath.Join(key, rel))
		keys = append(keys, k)

		_, err = u.Upload(ctx, &s3.PutObjectInput{
			Bucket: &bucket,
			Body:   file,
			Key:    &k,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil
		}

		fmt.Println("Uploaded", strings.Replace(k, key+"/", "", 1))

		return nil
	})
	if err != nil {
		return nil, err
	}

	return keys, nil
}

func Download(ctx context.Context, s3Client *s3.Client, bucket string, key string, dst string) ([]string, error) {
	d := manager.NewDownloader(s3Client)
	var keys []string

	result, err := s3Client.ListObjects(ctx, &s3.ListObjectsInput{
		Bucket: &bucket,
		Prefix: &key,
	})
	if err != nil {
		return nil, err
	}

	for _, c := range result.Contents {
		path := filepath.Join(dst, filepath.FromSlash(strings.Replace(*c.Key, key, "", 1)))
		_ = os.MkdirAll(filepath.Dir(path), 0755)
		file, err := os.Create(path)
		if err != nil {
			return nil, err
		}

		defer file.Close()

		k := *c.Key
		keys = append(keys, k)

		n, err := d.Download(ctx, file, &s3.GetObjectInput{
			Bucket: &bucket,
			Key:    &k,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}

		fmt.Println("Downloaded", strings.Replace(k, key+"/", "", 1), n, "bytes")
	}

	return keys, nil
}
