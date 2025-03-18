package handlers

import (
	"GoMldy/utils"
	"context"
	"encoding/json"
	"github.com/lrstanley/go-ytdlp"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

type VideoRequest struct {
	Url            string `json:"url"`
	RecaptchaToken string `json:"recaptchaToken"`
}

type CAPTCHARequest struct {
	Secret   string `json:"secret"`
	Response string `json:"response"`
}

type RecaptchaResponse struct {
	Success     bool      `json:"success"`
	Score       float64   `json:"score"`
	Action      string    `json:"action"`
	ChallengeTS time.Time `json:"challenge_ts"`
	Hostname    string    `json:"hostname"`
	ErrorCodes  []string  `json:"error-codes"`
}

func isCaptchaValid(token string) bool {
	data := url.Values{
		"secret":   {os.Getenv("RECAPTCHA_SECRET_KEY")},
		"response": {token},
	}

	resp, err := http.PostForm(
		"https://www.google.com/recaptcha/api/siteverify",
		data,
	)
	if err != nil {
		log.Println("Error creating request:", err)
		return false
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {

		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body: ", err)
		return false
	}
	response := &RecaptchaResponse{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		log.Println("Read error: got invalid JSON: ", err)
		return false
	}

	//log.Println(response)

	if !response.Success {
		log.Println("Recaptcha validation failed: ", response.ErrorCodes)
		return false
	}
	if response.Score < 0.3 {
		log.Println("Recaptcha score too low: ", response.Score)
		return false
	}

	return true
}

func Download(w http.ResponseWriter, r *http.Request) {
	video := &VideoRequest{}
	err := json.NewDecoder(r.Body).Decode(video)
	if err != nil {
		log.Println("Failed to decode video request")
		http.Error(w, "Failed to decode video request", http.StatusInternalServerError)
		return
	}
	//log.Println("video: ", video)

	if !isCaptchaValid(video.RecaptchaToken) {
		http.Error(w, "Recaptcha token invalid or request was not sent by a human!", http.StatusForbidden)
		return
	}

	origin := r.Header.Get("Origin")
	if !utils.RegexCORS(origin) {
		http.Error(w, "Unauthorized origin", http.StatusForbidden)
		return
	}

	limited, err := utils.CheckRateLimit(r.RemoteAddr)
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "Error while checking rate limit", http.StatusInternalServerError)
		return
	}
	if limited {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(
			w,
			"You are being rate limited! Please try again tomorrow or in ~24hours :3",
			http.StatusTooManyRequests,
		)
		return
	}

	//log.Printf("video url: %s, video object: %s", video.Url, video)

	filenameChan := make(chan string)

	tempDir, err := os.MkdirTemp("", "ytdl-temp")
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
		return
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			log.Println("Failed to remove temp directory: ", path)
		}
	}(tempDir)

	go func() {
		ytdlp.MustInstall(context.TODO(), nil)

		dl := ytdlp.New().
			Format("bestaudio").
			ExtractAudio().
			EmbedMetadata().
			AudioFormat("mp3").
			Output(filepath.Join(tempDir, "%(title)s.%(ext)s"))

		_, err = dl.Run(context.TODO(), video.Url)
		if err != nil {
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, "Download failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		files, err := filepath.Glob(filepath.Join(tempDir, "*.mp3"))
		if err != nil || len(files) == 0 {
			w.Header().Set("Content-Type", "text/plain")
			http.Error(w, "Failed to find downloaded file", http.StatusInternalServerError)
			return
		}
		tempFile := files[0]
		filename := filepath.Base(tempFile)

		filenameChan <- filename
	}()

	filename := <-filenameChan

	encodedFilename := url.PathEscape(filename)

	w.Header().Set("Content-Disposition", `attachment; filename="`+encodedFilename+`"`)
	w.Header().Set("Content-Type", "audio/mpeg; charset=utf-8")

	file, err := os.Open(filepath.Join(tempDir, filename))
	if err != nil {
		w.Header().Set("Content-Type", "text/plain")
		http.Error(w, "Failed to open downloaded file", http.StatusInternalServerError)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Failed to close file: %s: %s", file.Name(), err.Error())
		}
	}(file)

	_, err = io.Copy(w, file)
	if err != nil {
		log.Println("Error streaming file: ", err.Error())
		return
	}

	err = utils.IncreaseDlCount(r.RemoteAddr)
	if err != nil {
		log.Println("Error increasing dl count: ", err.Error())
		return
	}
}
