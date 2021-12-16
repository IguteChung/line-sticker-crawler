package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	pagesToParse  = 3
	packageHost   = "https://store.line.me/stickershop/showcase/top/zh-Hant"
	packagePrefix = `<img src="https://stickershop.line-scdn.net/stickershop/v1/product/`
	packageSuffix = `/LINEStorePC/thumbnail_shop.png;compress=true"`
	stickerHost   = "https://store.line.me/stickershop/product/%s/zh-Hant"
	stickerPrefix = `<span class="mdCMN09Image" style="background-image:url(https://stickershop.line-scdn.net/stickershop/v1/sticker/`
	stickerSuffix = `/iPhone/sticker@2x.png);"></span>`
	stickerImage  = "https://stickershop.line-scdn.net/stickershop/v1/sticker/%s/iPhone/sticker@2x.png"
)

func main() {
	if err := parseAll(); err != nil {
		log.Fatal(err)
	}
}

func parseAll() error {
	// parse all pages to get package ids.
	packageIDs := []string{}
	for i := 1; i <= pagesToParse; i++ {
		s, err := parsePackages(i)
		if err != nil {
			return fmt.Errorf("failed to parse page %d, %v", i, err)
		}
		packageIDs = append(packageIDs, s...)
	}

	// parse all stickers by package ids.
	stickerIDs := map[string][]string{}
	for _, id := range packageIDs {
		s, err := parseStickers(id)
		if err != nil {
			return fmt.Errorf("failed to parse sticker for package %s: %v", id, err)
		}
		stickerIDs[id] = s
	}

	// download the images by sticker ids.
	for packageID, stickerIDs := range stickerIDs {
		for _, stickerID := range stickerIDs {
			if err := downloadSticker(packageID, stickerID); err != nil {
				return fmt.Errorf("failed to download sticker %s: %v", packageID, err)
			}
		}
	}

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

func parsePackages(page int) ([]string, error) {
	// parse host url.
	u, err := url.Parse(packageHost)
	if err != nil {
		return nil, fmt.Errorf("failed to parse host %s: %v", packageHost, err)
	}

	// append page parameter.
	v := url.Values{
		"page": []string{fmt.Sprint(page)},
	}
	u.RawQuery = v.Encode()

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
	packageIDs := []string{}
	for _, s := range strings.Split(string(b), "\n") {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, packagePrefix) && strings.HasSuffix(s, packageSuffix) {
			packageIDs = append(packageIDs, s[len(packagePrefix):len(s)-len(packageSuffix)])
		}
	}
	fmt.Printf("page %d crawled\n", page)

	return packageIDs, nil
}

func downloadSticker(packageID, stickerID string) error {
	// skip download if zip exist.
	stickerPath := filepath.Join("../output", fmt.Sprintf("%s-%s.png", packageID, stickerID))
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
