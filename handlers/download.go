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
	"os/exec"
	"strings"
	"time"
)

type VideoRequest struct {
	Url            string `json:"url"`
	RecaptchaToken string `json:"recaptchaToken"`
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

	log.Println(response)

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
		log.Println("Failed to decode video request:", err)
		http.Error(w, "Failed to decode video request", http.StatusInternalServerError)
		return
	}

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

	ytdlPath, err := ytdlp.Install(context.TODO(), nil)
	if err != nil {
		log.Println("Error installing youtlp plugin:", err)
		http.Error(w, "Error installing youtlp plugin", http.StatusInternalServerError)
		return
	}

	metadataCmd := exec.Command(ytdlPath.Executable,
		"--no-download",
		"--print-json",
		"--skip-download",
		video.Url,
	)

	metadataOutput, err := metadataCmd.Output()
	if err != nil {
		log.Println("Error fetching video metadata:", err)
		http.Error(w, "Failed to fetch video information", http.StatusInternalServerError)
		return
	}

	var videoInfo struct {
		Title    string `json:"title"`
		Uploader string `json:"uploader"`
	}
	if err := json.Unmarshal(metadataOutput, &videoInfo); err != nil {
		log.Println("Error parsing video metadata:", err)
		http.Error(w, "Failed to parse video information", http.StatusInternalServerError)
		return
	}

	log.Println("Metadata fetched - Title:", videoInfo.Title, "Uploader:", videoInfo.Uploader)
	//Extract title and artist from the info

	filename := videoInfo.Title + ".mp3"
	filename = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) {
			return '-'
		}
		return r
	}, filename)

	encodedFilename := url.PathEscape(filename)
	w.Header().Set("Content-Disposition", `attachment; filename="`+encodedFilename+`"`)
	w.Header().Set("Content-Type", "audio/mpeg")

	log.Println("Executing cmd")
	ytdlCmd := exec.Command(ytdlPath.Executable,
		"--format", "bestaudio[ext!=webm]", // Pick best MP3-compatible format
		"--no-cache-dir",
		"--output", "-",
		video.Url,
	)

	ffmpegCmd := exec.Command("ffmpeg",
		"-i", "pipe:0", // Read from yt-dlp output
		"-vn",         // Remove any video
		"-ab", "320k", // High-quality audio
		"-ar", "48000", // Standard sampling rate
		"-metadata", "title="+videoInfo.Title,
		"-metadata", "artist="+videoInfo.Uploader,
		"-f", "mp3", // Output format MP3
		"pipe:1", // Stream to client
	)

	// Connect yt-dlp -> FFmpeg
	ytdlOut, err := ytdlCmd.StdoutPipe()
	if err != nil {
		log.Println("Error creating yt-dlp pipe:", err)
		http.Error(w, "Failed to process video", http.StatusInternalServerError)
		return
	}
	ffmpegCmd.Stdin = ytdlOut
	ffmpegCmd.Stdout = w

	// Start yt-dlp
	if err := ytdlCmd.Start(); err != nil {
		log.Println("Error starting yt-dlp:", err)
		http.Error(w, "Failed to start yt-dlp", http.StatusInternalServerError)
		return
	}

	// Start FFmpeg
	if err := ffmpegCmd.Run(); err != nil {
		log.Println("Error running FFmpeg:", err)
		http.Error(w, "Failed to process audio", http.StatusInternalServerError)
		return
	}

	// Wait for yt-dlp to finish
	if err := ytdlCmd.Wait(); err != nil {
		log.Println("yt-dlp finished with error:", err)
	}

	err = utils.IncreaseDlCount(r.RemoteAddr)
	if err != nil {
		log.Println("Error increasing download count:", err)
		return
	}
}
