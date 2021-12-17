package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"githb.com/igutechung/line-sticker-crawler/src/data"
)

var (
	emotionType = []string{"Happy", "Sad", "Fearful", "Angry", "Surprised", "Disgusted"}
	outputDir   string
)

const (
	limit         = 5
	packageHost   = "https://store.line.me/api/search/sticker"
	stickerHost   = "https://store.line.me/stickershop/product/%s/zh-Hant"
	stickerPrefix = `<span class="mdCMN09Image" style="background-image:url(https://stickershop.line-scdn.net/stickershop/v1/sticker/`
	stickerSuffix = `/iPhone/sticker@2x.png);"></span>`
	stickerImage  = "https://stickershop.line-scdn.net/stickershop/v1/sticker/%s/iPhone/sticker@2x.png"
)

func init() {
	// get the path of main.go.
	_, f, _, _ := runtime.Caller(0)
	mainDir := filepath.Dir(f)
	outputDir = filepath.Join(mainDir, "..", "output")
}

func main() {
	if err := parseAll(); err != nil {
		log.Fatal(err)
	}
}

func parseAll() error {
	// parse all pages to get package ids.
	emotionPackages := map[string][]string{}
	for _, emotion := range emotionType {
		packages, err := parsePackages(emotion)
		if err != nil {
			return fmt.Errorf("failed to parse %s, %v", emotion, err)
		}
		emotionPackages[emotion] = append(emotionPackages[emotion], packages...)
	}

	// parse all stickers by package ids.
	stickerIDs := map[string][]string{}
	for _, packageIDs := range emotionPackages {
		for _, packageID := range packageIDs {
			stickers, err := parseStickers(packageID)
			if err != nil {
				return fmt.Errorf("failed to parse sticker for package %s: %v", packageID, err)
			}
			stickerIDs[packageID] = stickers
		}
	}

	// download the images by sticker ids.
	for packageID, stickerIDs := range stickerIDs {
		for _, stickerID := range stickerIDs {
			if err := downloadSticker(packageID, stickerID); err != nil {
				return fmt.Errorf("failed to download sticker %s: %v", packageID, err)
			}
		}
	}

	// use a mapping from image name to emotion to true/false.
	m := map[string]map[string]bool{}
	for emotion, packageIDs := range emotionPackages {
		for _, packageID := range packageIDs {
			for _, stickerID := range stickerIDs[packageID] {
				// check the image name is in map and insert default emotions.
				image := filename(packageID, stickerID)
				if _, ok := m[image]; !ok {
					m[image] = map[string]bool{}
					for _, emotion := range emotionType {
						m[image][emotion] = false
					}
				}

				// set the given emotion to true.
				m[image][emotion] = true
			}
		}
	}

	// generate csv rows.
	rows := [][]string{}
	for image, emotions := range m {
		for emotion, ok := range emotions {
			rows = append(rows, []string{image, emotion, data.FormatBool(ok)})
		}
	}

	// write the csv.
	csvPath := filepath.Join(outputDir, "label.csv")
	if err := data.WriteCSV(csvPath, rows); err != nil {
		return fmt.Errorf("failed to write csv to %s: %v", csvPath, err)
	}
	fmt.Printf("%s generated!", csvPath)

	return nil
}

func parseStickers(id string) ([]string, error) {
	// parse host url.
	u, err := url.Parse(fmt.Sprintf(stickerHost, id))
	if err != nil {
		return nil, fmt.Errorf("failed to parse sticker host %s: %v", id, err)
	}

	// get the html.
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %v", u.String(), err)
	}
	defer resp.Body.Close()

	// read all conents.
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}

	// find all package ids.
	stickerIDs := []string{}
	for _, s := range strings.Split(string(b), "\n") {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, stickerPrefix) && strings.HasSuffix(s, stickerSuffix) {
			stickerIDs = append(stickerIDs, s[len(stickerPrefix):len(s)-len(stickerSuffix)])
		}
	}
	fmt.Printf("package %s crawled\n", id)

	return stickerIDs, nil
}

func parsePackages(emotion string) ([]string, error) {
	// parse host url.
	u, err := url.Parse(packageHost)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host %s: %v", packageHost, err)
	}

	// append page parameter.
	v := url.Values{
		"limit":         []string{fmt.Sprint(limit)},
		"query":         []string{emotion},
		"offset":        []string{"0"},
		"type":          []string{"ALL"},
		"includeFacets": []string{"false"},
	}
	u.RawQuery = v.Encode()

	// get the html.
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to get %s: %v", u.String(), err)
	}
	defer resp.Body.Close()

	// read all conents and decode to json.
	stickerPackages := new(data.StickerQuery)
	d := json.NewDecoder(resp.Body)
	if err := d.Decode(stickerPackages); err != nil {
		return nil, fmt.Errorf("failed to decode body: %v", err)
	}

	// convert response to slice.
	packageIDs := make([]string, len(stickerPackages.Items))
	for i, item := range stickerPackages.Items {
		packageIDs[i] = item.ID
	}
	fmt.Printf("[%s] package crawled %d\n", emotion, limit)

	return packageIDs, nil
}

func downloadSticker(packageID, stickerID string) error {
	// mkdir if necessary.
	stickerPath := filepath.Join(outputDir, "image", filename(packageID, stickerID))
	if err := os.MkdirAll(filepath.Dir(stickerPath), 0755); err != nil {
		return fmt.Errorf("failed to mkdir %s: %v", stickerPath, err)
	}

	// skip download if zip exist.
	if _, err := os.Stat(stickerPath); err == nil {
		fmt.Printf("%s existed!\n", stickerPath)
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check %s exist: %v", stickerPath, err)
	}

	// replace the sticker pack url by package id.
	u, err := url.Parse(fmt.Sprintf(stickerImage, stickerID))
	if err != nil {
		return fmt.Errorf("failed to parse id url %s: %v", stickerPath, err)
	}

	// download the zip.
	resp, err := http.Get(u.String())
	if err != nil {
		return fmt.Errorf("failed to get %s: %v", u.String(), err)
	}
	defer resp.Body.Close()

	//store the zip.
	out, err := os.Create(stickerPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", stickerPath, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("failed to copy to %s: %v", stickerPath, err)
	}
	fmt.Printf("%s download successfully.\n", stickerPath)

	return nil
}

func filename(packageID, stickerID string) string {
	return fmt.Sprintf("%s-%s.png", packageID, stickerID)
}
