package repository

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/debemdeboas/the-archive/internal/cache"
	"github.com/debemdeboas/the-archive/internal/model"
)

type S3PostRepository struct { // implements PostRepository
	client *s3.Client

	postsCache       *cache.Cache[string, *model.Post]
	postsCacheSorted []model.Post

	reloadNotifier func(model.PostId)
}

func NewS3PostRepository(accessKeyId, accessKeySecret, baseEndpoint string) *S3PostRepository {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		repoLogger.Fatal().Err(err).Msg("Error initializing S3 client")
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(baseEndpoint)
	})

	return &S3PostRepository{
		client: client,

		postsCache: cache.NewCache[string, *model.Post](),
	}
}

func (r *S3PostRepository) SetReloadNotifier(notifier func(model.PostId)) {
	r.reloadNotifier = notifier
}

func (r *S3PostRepository) notifyPostReload(postID model.PostId) {
	if r.reloadNotifier != nil {
		r.reloadNotifier(postID)
	}
}

func (r *S3PostRepository) Init() {
	posts, postMap, err := r.GetPosts()
	if err != nil {
		repoLogger.Fatal().Err(err).Msg("Error initializing posts")
	}

	r.postsCacheSorted = posts
	r.postsCache.SetTo(postMap)

	go r.ReloadPosts()
}

func (r *S3PostRepository) GetPostList() []model.Post {
	return r.postsCacheSorted
}

func (r *S3PostRepository) GetPosts() ([]model.Post, map[string]*model.Post, error) {
	entries, err := r.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{})
	if err != nil {
		return nil, nil, err
	}

	var posts []model.Post
	postsMap := make(map[string]*model.Post)
	for _, entry := range entries.Contents {
		fmt.Println(entry)
		// if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
		// 	name := strings.TrimSuffix(entry.Name(), ".md")

		// 	mdContent, err := r.ReadPost(name)
		// 	if err != nil {
		// 		return nil, nil, err
		// 	}

		// 	fileInfo, err := entry.Info()
		// 	if err != nil {
		// 		return nil, nil, err
		// 	}

		// 	post := model.Post{
		// 		Title:         name,
		// 		Path:          name,
		// 		MDContentHash: util.ContentHash(mdContent),
		// 		ModifiedDate:  fileInfo.ModTime(),
		// 	}

		// 	posts = append(posts, post)
		// 	postsMap[name] = &post
		// }
	}

	slices.SortStableFunc(posts, func(a, b model.Post) int {
		return -a.ModifiedDate.Compare(b.ModifiedDate)
	})

	return posts, postsMap, nil
}

func (r *S3PostRepository) ReadPost(path any) ([]byte, error) {
	return path.([]byte), nil
}

func (r *S3PostRepository) ReloadPosts() {
	for {
		posts, postMap, err := r.GetPosts()
		if err != nil {
			repoLogger.Error().Err(err).Msg("Error reloading posts")
		} else {
			for _, post := range r.postsCacheSorted {
				if newPost, ok := postMap[post.Path]; ok {
					if newPost.MDContentHash != post.MDContentHash {
						repoLogger.Info().
							Str("post_id", string(post.Id)).
							Str("title", post.Title).
							Msg("Reloading post")
						go r.notifyPostReload(model.PostId(post.Path))
					}
				}
			}

			r.postsCacheSorted = posts
			r.postsCache.SetTo(postMap)
		}
		time.Sleep(5 * time.Second)
	}
}
