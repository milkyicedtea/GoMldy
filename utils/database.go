package utils

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
	"strings"
)

var pgxPool *pgxpool.Pool

func InitDb() {
	var err error

	pgxPool, err = pgxpool.New(context.Background(), os.Getenv("MELODY_PSQL_URL"))
	if err != nil {
		log.Fatal(err)
	}
}

type RateCheckResult struct {
	DownloadCount int `json:"download_count"`
}

func CheckRateLimit(ip string) (bool, error) {
	//log.Println("Begin checking with IP: ", ip)
	db, err := pgxPool.Acquire(context.Background())
	if err != nil {
		log.Println("Error while connecting to database: ", err)
		return true, err
	}
	defer db.Release()

	//log.Println("Hashing the IP: ", strings.Split(ip, ":")[0])

	h := sha256.New()
	h.Write([]byte(strings.Split(ip, ":")[0]))
	hashedIP := hex.EncodeToString(h.Sum(nil))
	//log.Println("Hashed IP: ", hashedIP)

	var downloadCount int

	//log.Println("Querying row for download count")
	err = db.QueryRow(
		context.Background(),
		`select download_count from download_rate_limits where hashed_ip = $1`,
		hashedIP,
	).Scan(&downloadCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Println("Error while querying row for download rate limits: ", err)
			downloadCount = 0
			return false, nil
		}
		log.Println("Error while querying database: ", err)
		return true, err
	}

	log.Println("Download rate limit check result: ", downloadCount)

	//log.Println("Checking if count is above 5")
	if downloadCount >= 5 {
		return true, nil
	}

	return false, nil
}

func IncreaseDlCount(ip string) error {
	//log.Println("Begin count increase with IP: ", ip)
	dbTx, err := pgxPool.Acquire(context.Background())
	if err != nil {
		log.Println("Error while beginning tx: ", err)
		return err
	}
	defer dbTx.Release()

	//log.Println("Hashing the IP: ", strings.Split(ip, ":")[0])

	h := sha256.New()
	h.Write([]byte(strings.Split(ip, ":")[0]))
	hashedIP := hex.EncodeToString(h.Sum(nil))
	//log.Println("Hashed IP: ", hashedIP)

	log.Println("Increasing download count")
	_, err = dbTx.Query(
		context.Background(),
		`insert into download_rate_limits (hashed_ip, download_count, last_reset)
		values ($1, 1, current_timestamp)
		on conflict (hashed_ip)
		do update set download_count = download_rate_limits.download_count + 1`,
		hashedIP,
	)
	if err != nil {
		log.Println("Error while updating download_count: ", err)
		return err
	}

	return nil
}
