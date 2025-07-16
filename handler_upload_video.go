package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

type VideoInfo struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading video", videoID, "by user", userID)

	maxMemory := int64(10 << 30)

	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error retrieving video", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse content type", err)
		return
	}

	if mediaType != "video/mp4" {
		err = errors.New("unsupported file type")
		respondWithError(w, http.StatusBadRequest, "Unsupported file type", err)
	}

	temp_file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error creating temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer temp_file.Close()

	_, err = io.Copy(temp_file, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error writing to temp file", err)
		return
	}

	_, err = temp_file.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error reseting temp file pointer", err)
		return
	}

	aspectRatio, err := getVideoAspectRatio(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting video aspect ratio", err)
		return
	}

	newPath, err := processVideoForFastStart(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error processing video for fast start", err)
		return
	}

	processedVideo, err := os.Open(newPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error getting processed video", err)
		return
	}
	defer processedVideo.Close()

	nameBytes := make([]byte, 32)
	rand.Read(nameBytes)
	randomName := base64.RawURLEncoding.EncodeToString(nameBytes)

	extension := strings.Split(mediaType, "/")[1]
	filekey := aspectRatio + "/" + strings.Join([]string{randomName, extension}, ".")

	putObjParams := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filekey,
		Body:        processedVideo,
		ContentType: &mediaType,
	}

	cfg.s3Client.PutObject(context.Background(), &putObjParams)

	stringsForURL := []string{
		"https://" + cfg.s3Bucket,
		"s3",
		cfg.s3Region,
		"amazonaws",
		"com",
	}
	vidURL := strings.Join(stringsForURL, ".") + "/" + filekey

	video.VideoURL = &vidURL

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
