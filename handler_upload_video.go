package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

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

	fmt.Println("uploading video file", videoID, "by user", userID)

	const maxMemory = 10 << 30
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	media := header.Header.Get("Content-Type")
	mediaMimeType, _, err := mime.ParseMediaType(media)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse media type", err)
		return
	}
	fmt.Println(mediaMimeType)
	if mediaMimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Video must be mp4 format", err)
		return
	}

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video metadata", err)
		return
	}
	fmt.Printf("Debug: After GetVideo, videoMetaData = %+v\n", videoMetaData)

	if userID != videoMetaData.UserID {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	randomBytes := make([]byte, 32)
	_, err = rand.Read(randomBytes)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating rand", err)
		return
	}
	encodedRandNum := base64.RawURLEncoding.EncodeToString(randomBytes)

	serverFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}
	defer os.Remove(serverFile.Name())
	defer serverFile.Close()

	io.Copy(serverFile, file)

	serverFile.Seek(0, io.SeekStart)

	fastStartFilePath, err := processVideoForFastStart(serverFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to process video for fast start", err)
		return
	}

	aspect_ratio, err := getVideoAspectRatio(serverFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to determine video aspect ratio", err)
		return
	}

	fastStartFile, err := os.Open(fastStartFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to open fast start file", err)
		return
	}

	defer fastStartFile.Close()
	defer os.Remove(fastStartFilePath)

	var s3Key string

	if aspect_ratio == "16:9" {
		s3Key = "landscape"
	} else if aspect_ratio == "9:16" {
		s3Key = "portrait"
	} else {
		s3Key = "other"
	}

	s3Key = s3Key + "/" + encodedRandNum + ".mp4"

	s3Params := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &s3Key,
		Body:        fastStartFile,
		ContentType: &mediaMimeType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3Params)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload video", err)
		return
	}

	videoURL := fmt.Sprintf("%v%v", cfg.s3CfDistribution, s3Key)
	videoMetaData.VideoURL = &videoURL

	err = cfg.db.UpdateVideo(videoMetaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}
