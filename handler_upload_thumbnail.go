package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {

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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}

	defer file.Close()

	media := header.Header.Get("Content-Type")
	mediaMimeType, _, err := mime.ParseMediaType(media)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse media type", err)
	}
	if mediaMimeType != "image/jpeg" && mediaMimeType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "Thumbnail must be JPEG or PNG", err)
		return
	}

	videoMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to find video metadata", err)
		return
	}

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

	thumbnailURL := filepath.Join(cfg.assetsRoot, encodedRandNum)
	thumbnailURL += "." + strings.Split(media, "/")[1]

	serverFile, err := os.Create(thumbnailURL)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to create file", err)
		return
	}

	defer serverFile.Close()

	io.Copy(serverFile, file)

	thumbnailURL = fmt.Sprintf("http://localhost:%v/%v", cfg.port, thumbnailURL)

	videoMetaData.ThumbnailURL = &thumbnailURL

	err = cfg.db.UpdateVideo(videoMetaData)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update database", err)
		return
	}

	respondWithJSON(w, http.StatusOK, videoMetaData)
}
