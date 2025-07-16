package main

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func getVideoAspectRatio(filepath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)

	var buff bytes.Buffer
	cmd.Stdout = &buff

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	var vidSize VideoInfo
	if err = json.Unmarshal(buff.Bytes(), &vidSize); err != nil {
		return "", err
	}

	width := float64(vidSize.Streams[0].Width)
	height := float64(vidSize.Streams[0].Height)

	ratio := width / height
	landscape := float64(16) / float64(9)
	portrait := float64(9) / float64(16)

	if math.Abs(landscape-ratio) < 0.1 {
		return "landscape", nil
	} else if math.Abs(portrait-ratio) < 0.1 {
		return "portrait", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filepath string) (string, error) {
	outputPath := filepath + ".processing"
	cmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)

	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return outputPath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	params := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	signedURL, err := presignClient.PresignGetObject(context.Background(), &params, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return signedURL.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	params := strings.Split(*video.VideoURL, ",")
	bucket := params[0]
	key := params[1]

	newURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 20*time.Minute)
	if err != nil {
		return database.Video{}, err
	}

	video.VideoURL = &newURL

	return video, nil
}
