package storage

import (
	"context"
	"fmt"
	"log"

	"backend/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	s3config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type StorageService interface {
	DeletePrefix(ctx context.Context, prefix string) error
	DeleteScene(ctx context.Context, sceneID string) error
	DeleteTour(ctx context.Context, tourID string, sceneIDs []string) error
}

type R2Service struct {
	client *s3.Client
	bucket string
}

func NewR2Service(cfg config.Config) *R2Service {
	if cfg.R2Bucket == "" || cfg.R2Key == "" || cfg.R2Secret == "" || cfg.R2Endpoint == "" {
		log.Println("R2 storage is not fully configured; R2 file deletion will be skipped")
		return nil
	}

	awsCfg, err := s3config.LoadDefaultConfig(context.Background(),
		s3config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.R2Key, cfg.R2Secret, "")),
		s3config.WithRegion(cfg.R2Region),
	)
	if err != nil {
		log.Printf("Failed to initialize R2 AWS config: %v", err)
		return nil
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.R2Endpoint)
		o.UsePathStyle = cfg.R2PathStyle
	})

	return &R2Service{
		client: client,
		bucket: cfg.R2Bucket,
	}
}

func (r *R2Service) DeletePrefix(ctx context.Context, prefix string) error {
	if r == nil || r.client == nil {
		return nil
	}

	var continuationToken *string
	totalDeleted := 0

	for {
		listResp, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(r.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return fmt.Errorf("failed to list objects with prefix %s: %w", prefix, err)
		}

		if len(listResp.Contents) == 0 {
			break
		}

		var toDelete []types.ObjectIdentifier
		for _, item := range listResp.Contents {
			if item.Key != nil {
				toDelete = append(toDelete, types.ObjectIdentifier{Key: item.Key})
			}
		}

		if len(toDelete) > 0 {
			_, err := r.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
				Bucket: aws.String(r.bucket),
				Delete: &types.Delete{
					Objects: toDelete,
					Quiet:   aws.Bool(true),
				},
			})
			if err != nil {
				return fmt.Errorf("failed to delete objects with prefix %s: %w", prefix, err)
			}
			totalDeleted += len(toDelete)
		}

		if listResp.IsTruncated != nil && *listResp.IsTruncated {
			continuationToken = listResp.NextContinuationToken
		} else {
			break
		}
	}

	if totalDeleted > 0 {
		log.Printf("Deleted %d objects from R2 bucket %s with prefix %s", totalDeleted, r.bucket, prefix)
	}
	return nil
}

func (r *R2Service) DeleteScene(ctx context.Context, sceneID string) error {
	if sceneID == "" {
		return nil
	}
	prefix := fmt.Sprintf("scenes/%s/", sceneID)
	return r.DeletePrefix(ctx, prefix)
}

func (r *R2Service) DeleteTour(ctx context.Context, tourID string, sceneIDs []string) error {
	if r == nil || r.client == nil {
		return nil
	}

	// 1. Delete tour models and overlays: tours/{tourId}/
	tourPrefix := fmt.Sprintf("tours/%s/", tourID)
	if err := r.DeletePrefix(ctx, tourPrefix); err != nil {
		log.Printf("Warning: failed to delete tour assets from R2: %v", err)
	}

	// 2. Delete all scenes belonging to this tour
	for _, sceneID := range sceneIDs {
		if err := r.DeleteScene(ctx, sceneID); err != nil {
			log.Printf("Warning: failed to delete scene %s from R2: %v", sceneID, err)
		}
	}

	return nil
}
