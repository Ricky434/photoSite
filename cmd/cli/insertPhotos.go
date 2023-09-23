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

	if c.path == "" || c.storageDir == "" {
		c.fs.Usage()
		fmt.Println()

		return errors.New("No photos or storage path provided")
	}

	return nil
}

func (c *insertPhotosCommand) Run(db *sql.DB) error {
	fmt.Println("flag:", c.path)
	fmt.Println("flag:", c.event)
	fmt.Println("flag:", c.storageDir)

	m := models.New(db)
	return c.recursiveInsert(&m, c.path)
}

func (c *insertPhotosCommand) recursiveInsert(m *models.Models, photo_path string) error {
	stat, err := os.Stat(photo_path)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		dir, err := os.ReadDir(photo_path)
		if err != nil {
			return err
		}

		for _, f := range dir {
			err = c.recursiveInsert(m, path.Join(photo_path, f.Name()))
			if err != nil {
				return err
			}
		}
		return nil
	}

	if !slices.Contains(models.ImageExtensions, strings.ToLower(path.Ext(photo_path))) {
		fmt.Printf("Skipping invalid image: %s\n", photo_path)
		return nil
	}

	return c.insertPhoto(m, photo_path)
}

func (c *insertPhotosCommand) insertPhoto(m *models.Models, photo_path string) error {
	// Extract metadata from photo
	var ExiftoolOut []struct {
		Latitude  *float32   `json:"GPSLatitude"`
		Longitude *float32   `json:"GPSLongitude"`
		TakenAt   *time.Time `json:"DateTimeOriginal"`
	}

	cmd := exec.Command(
		"exiftool",
		"-TAG", "-GPSLatitude#", "-GPSLongitude#", "-DateTimeOriginal",
		"-j",
		"-d", "%Y-%m-%dT%H:%M:%SZ",
		photo_path,
	)

	stdout, err := cmd.Output()
	if err != nil {
		return err
	}

	err = json.Unmarshal(stdout, &ExiftoolOut)
	if err != nil {
		return err
	}

	// Retrieve event id if event name provided
	var eventID *int32

	if c.event != "" {
		event, err := m.Events.GetByName(c.event)
		if err != nil {
			if errors.Is(err, models.ErrRecordNotFound) {
				return fmt.Errorf("Event does not exist")
			}
			return err
		}

		e := int32(event.ID)
		eventID = &e
	}

	photoExtension := strings.ToLower(path.Ext(photo_path))

	// Insert photo data in db
	photo := &models.Photo{
		TakenAt:   ExiftoolOut[0].TakenAt,
		Latitude:  ExiftoolOut[0].Latitude,
		Longitude: ExiftoolOut[0].Longitude,
		Event:     eventID,
		Extension: photoExtension,
	}

	err = m.Photos.Insert(photo)
	if err != nil {
		return err
	}

	// Copy photo in storage
	// Open photo
	source, err := os.Open(photo_path)
	if err != nil {
		m.Photos.Delete(photo.ID)
		return err
	}
	defer source.Close()

	// Create event directory if not exists
	eventDir := path.Join(c.storageDir, c.event)
	if _, err := os.Stat(eventDir); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(eventDir, os.ModePerm)
		if err != nil {
			m.Photos.Delete(photo.ID)
			return err
		}
	}

	// Open destination
	// Rischio di sovrascrivere? Dovrei preservare le estensioni?
	destination, err := os.Create(path.Join(eventDir, fmt.Sprintf("%s%s", photo.ID, photoExtension)))
	if err != nil {
		m.Photos.Delete(photo.ID)
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		m.Photos.Delete(photo.ID)
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
	c.fs.StringVar(&c.storageDir, "storage-dir", "./storage/photos", "Photos storage directory")

	return c
}
