package main

import (
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"openreplay/backend/pkg/db/postgres"
	. "openreplay/backend/pkg/messages"
	"openreplay/backend/pkg/token"
)

const FILES_SIZE_LIMIT int64 = 1e7 // 10Mb

func startSessionHandlerIOS(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Token          string  `json:"token"`
		ProjectKey     *string `json:"projectKey"`
		TrackerVersion string  `json:"trackerVersion"`
		RevID          string  `json:"revID"`
		UserUUID       *string `json:"userUUID"`
		UserOSVersion  string  `json:"userOSVersion"`
		UserDevice     string  `json:"userDevice"`
		Timestamp      uint64  `json:"timestamp"`
	}
	type response struct {
		Token           string   `json:"token"`
		ImagesHashList  []string `json:"imagesHashList"`
		UserUUID        string   `json:"userUUID"`
		BeaconSizeLimit int64    `json:"beaconSizeLimit"`
		SessionID       string   `json:"sessionID"`
	}
	startTime := time.Now()
	req := &request{}
	body := http.MaxBytesReader(w, r.Body, JSON_SIZE_LIMIT)
	defer body.Close()
	if err := json.NewDecoder(body).Decode(req); err != nil {
		responseWithError(w, http.StatusBadRequest, err)
		return
	}

	if req.ProjectKey == nil {
		responseWithError(w, http.StatusForbidden, errors.New("ProjectKey value required"))
		return
	}

	p, err := pgconn.GetProjectByKey(*req.ProjectKey)
	if err != nil {
		if postgres.IsNoRowsErr(err) {
			responseWithError(w, http.StatusNotFound, errors.New("Project doesn't exist or is not active"))
		} else {
			responseWithError(w, http.StatusInternalServerError, err) // TODO: send error here only on staging
		}
		return
	}
	userUUID := getUUID(req.UserUUID)
	tokenData, err := tokenizer.Parse(req.Token)

	if err != nil { // Starting the new one
		dice := byte(rand.Intn(100)) // [0, 100)
		if dice >= p.SampleRate {
			responseWithError(w, http.StatusForbidden, errors.New("cancel"))
			return
		}

		ua := uaParser.ParseFromHTTPRequest(r)
		if ua == nil {
			responseWithError(w, http.StatusForbidden, errors.New("browser not recognized"))
			return
		}
		sessionID, err := flaker.Compose(uint64(startTime.UnixMilli()))
		if err != nil {
			responseWithError(w, http.StatusInternalServerError, err)
			return
		}
		// TODO: if EXPIRED => send message for two sessions association
		expTime := startTime.Add(time.Duration(p.MaxSessionDuration) * time.Millisecond)
		tokenData = &token.TokenData{sessionID, expTime.UnixMilli()}

		country := geoIP.ExtractISOCodeFromHTTPRequest(r)

		// The difference with web is mostly here:
		producer.Produce(TOPIC_RAW_IOS, tokenData.ID, Encode(&IOSSessionStart{
			Timestamp:      req.Timestamp,
			ProjectID:      uint64(p.ProjectID),
			TrackerVersion: req.TrackerVersion,
			RevID:          req.RevID,
			UserUUID:       userUUID,
			UserOS:         "IOS",
			UserOSVersion:  req.UserOSVersion,
			UserDevice:     MapIOSDevice(req.UserDevice),
			UserDeviceType: GetIOSDeviceType(req.UserDevice),
			UserCountry:    country,
		}))
	}

	responseWithJSON(w, &response{
		Token:           tokenizer.Compose(*tokenData),
		UserUUID:        userUUID,
		SessionID:       strconv.FormatUint(tokenData.ID, 10),
		BeaconSizeLimit: BEACON_SIZE_LIMIT,
	})
}

func pushMessagesHandlerIOS(w http.ResponseWriter, r *http.Request) {
	sessionData, err := tokenizer.ParseFromHTTPRequest(r)
	if err != nil {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}
	pushMessages(w, r, sessionData.ID, TOPIC_RAW_IOS)
}

func pushLateMessagesHandlerIOS(w http.ResponseWriter, r *http.Request) {
	sessionData, err := tokenizer.ParseFromHTTPRequest(r)
	if err != nil && err != token.EXPIRED {
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}
	// Check timestamps here?
	pushMessages(w, r, sessionData.ID, TOPIC_RAW_IOS)
}

func imagesUploadHandlerIOS(w http.ResponseWriter, r *http.Request) {
	log.Printf("recieved imagerequest")

	sessionData, err := tokenizer.ParseFromHTTPRequest(r)
	if err != nil { // Should accept expired token?
		responseWithError(w, http.StatusUnauthorized, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, FILES_SIZE_LIMIT)
	defer r.Body.Close()
	err = r.ParseMultipartForm(1e6) // ~1Mb
	if err == http.ErrNotMultipart || err == http.ErrMissingBoundary {
		responseWithError(w, http.StatusUnsupportedMediaType, err)
		// } else if err == multipart.ErrMessageTooLarge // if non-files part exceeds 10 MB
	} else if err != nil {
		responseWithError(w, http.StatusInternalServerError, err) // TODO: send error here only on staging
	}

	if r.MultipartForm == nil {
		responseWithError(w, http.StatusInternalServerError, errors.New("Multipart not parsed"))
	}

	if len(r.MultipartForm.Value["projectKey"]) == 0 {
		responseWithError(w, http.StatusBadRequest, errors.New("projectKey parameter missing")) // status for missing/wrong parameter?
		return
	}

	prefix := r.MultipartForm.Value["projectKey"][0] + "/" + strconv.FormatUint(sessionData.ID, 10) + "/"

	for _, fileHeaderList := range r.MultipartForm.File {
		for _, fileHeader := range fileHeaderList {
			file, err := fileHeader.Open()
			if err != nil {
				continue // TODO: send server error or accumulate successful files
			}
			key := prefix + fileHeader.Filename
			log.Printf("Uploading image... %v", key)
			go func() { //TODO: mime type from header
				if err := s3.Upload(file, key, "image/jpeg", false); err != nil {
					log.Printf("Upload ios screen error. %v", err)
				}
			}()
		}
	}

	w.WriteHeader(http.StatusOK)
}
