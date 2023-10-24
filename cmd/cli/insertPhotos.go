package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"sitoWow/internal/data/models"
	"slices"
	"strings"
	"time"
)

type insertPhotosCommand struct {
	path       string
	event      string
	storageDir string
	fs         *flag.FlagSet
}

func (c *insertPhotosCommand) Init(args []string) error {
	err := c.fs.Parse(args)
	if err != nil {
		return err
	}

	if c.event == "" || c.path == "" || c.storageDir == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("Not enough arguments provided")
	}

	return nil
}

func (c *insertPhotosCommand) Run(db *sql.DB) error {
	fmt.Println("flag:", c.path)
	fmt.Println("flag:", c.event)
	fmt.Println("flag:", c.storageDir)

	m := models.New(db)

	// Create photos and event directories if not present
	photosDir := path.Join(c.storageDir, "photos", c.event)
	if _, err := os.Stat(photosDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(photosDir, os.ModePerm)
		if err != nil {
			return err
		}
	}

	// Create thumbnails directory if not present
	thumbsDir := path.Join(c.storageDir, "thumbnails", c.event)
	if _, err := os.Stat(thumbsDir); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(thumbsDir, os.ModePerm)
		if err != nil {
			return err
		}
	}
	return c.recursiveInsert(&m, c.path)
}

func (c *insertPhotosCommand) recursiveInsert(m *models.Models, file_path string) error {
	stat, err := os.Stat(file_path)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		dir, err := os.ReadDir(file_path)
		if err != nil {
			return err
		}

		for _, f := range dir {
			err = c.recursiveInsert(m, path.Join(file_path, f.Name()))
			if err != nil {
				return err
			}
		}
		return nil
	}

	if slices.Contains(models.ImageExtensions, strings.ToLower(path.Ext(file_path))) {
		return c.insertFile(m, file_path, false)
	}

	if slices.Contains(models.VideoExtensions, strings.ToLower(path.Ext(file_path))) {
		return c.insertFile(m, file_path, true)
	}

	fmt.Printf("Skipping invalid image or video: %s\n", file_path)
	return nil
}

func (c *insertPhotosCommand) insertFile(m *models.Models, file_path string, isVideo bool) error {
	// Extract metadata from photo
	var ExiftoolOut []struct {
		Latitude        *float32   `json:"GPSLatitude"`
		Longitude       *float32   `json:"GPSLongitude"`
		TakenAt         *time.Time `json:"DateTimeOriginal"`
		TakenAtFallback *time.Time `json:"TrackCreateDate"`
	}

	cmd := exec.Command(
		"exiftool",
		"-TAG", "-GPSLatitude#", "-GPSLongitude#", "-DateTimeOriginal", "-TrackCreateDate",
		"-j",
		"-d", "%Y-%m-%dT%H:%M:%SZ",
		file_path,
	)

	stdout, err := cmd.Output()
	if err != nil {
		return err
	}

	err = json.Unmarshal(stdout, &ExiftoolOut)
	if err != nil {
		return err
	}

	// Retrieve event id
	event, err := m.Events.GetByName(c.event)
	if err != nil {
		if errors.Is(err, models.ErrRecordNotFound) {
			return fmt.Errorf("Event does not exist")
		}
		return err
	}

	eventID := event.ID

	// Insert file data in db
	photo := &models.Photo{
		FileName:  path.Base(file_path),
		TakenAt:   ExiftoolOut[0].TakenAt,
		Latitude:  ExiftoolOut[0].Latitude,
		Longitude: ExiftoolOut[0].Longitude,
		Event:     eventID,
	}

	if photo.TakenAt == nil && ExiftoolOut[0].TakenAtFallback != nil {
		photo.TakenAt = ExiftoolOut[0].TakenAtFallback
	}

	err = m.Photos.Insert(photo)
	if err != nil {
		return err
	}

	// Copy photo in storage
	// Open photo
	source, err := os.Open(file_path)
	if err != nil {
		m.Photos.Delete(photo.ID)
		return err
	}
	defer source.Close()

	eventDir := path.Join(c.storageDir, "photos", c.event)

	// Open destination
	destination, err := os.Create(path.Join(eventDir, photo.FileName))
	if err != nil {
		m.Photos.Delete(photo.ID)
		return err
	}
	defer destination.Close()

	fmt.Println("copying..")
	_, err = io.Copy(destination, source)
	if err != nil {
		m.Photos.Delete(photo.ID)
		return err
	}
	fmt.Println("copied")

	// Make thumbnail
	var magickCmd *exec.Cmd
	if !isVideo {
		magickCmd = exec.Command(
			"mogrify",
			"-auto-orient",
			"-path", path.Join(c.storageDir, "thumbnails", c.event),
			"-thumbnail", "500x500",
			file_path,
		)
	} else {
		magickCmd = exec.Command(
			"magick", "convert",
			"-resize", "500x500>",
			fmt.Sprintf("%s[1]", file_path),
			// Thumbnail for video is video filename(with extension)+".jpg"
			path.Join(c.storageDir, "thumbnails", c.event, fmt.Sprintf("%s%s", path.Base(file_path), ".jpg")),
		)
	}

	fmt.Println(magickCmd.Args)
	fmt.Println("thumbnailing..")
	_, err = magickCmd.Output()
	if err != nil {
		// Rollback
		m.Photos.Delete(photo.ID)
		destination.Close()
		os.Remove(destination.Name())
		return err
	}

	return nil
}

func (c *insertPhotosCommand) Name() string {
	return "insertPhotos"
}

func newInsertPhotosCommand() *insertPhotosCommand {
	c := &insertPhotosCommand{
		fs: flag.NewFlagSet("insertPhotos", flag.ContinueOnError),
	}
	c.fs.StringVar(&c.path, "path", "", "Photos location (folder or single file)")
	c.fs.StringVar(&c.event, "event", "", "Event name")
	c.fs.StringVar(&c.storageDir, "storage-dir", "./storage", "Photos storage directory")

	return c
}
