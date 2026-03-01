//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type meta struct {
	CompanyName      string `json:"company_name"`
	ProductName      string `json:"product_name"`
	FileDescription  string `json:"file_description"`
	InternalName     string `json:"internal_name"`
	OriginalFilename string `json:"original_filename"`
	LegalCopyright   string `json:"legal_copyright"`
	IconPath         string `json:"icon_path"`
}

type fileVersion struct {
	Major int `json:"Major"`
	Minor int `json:"Minor"`
	Patch int `json:"Patch"`
	Build int `json:"Build"`
}

type versionInfo struct {
	FixedFileInfo struct {
		FileVersion    fileVersion `json:"FileVersion"`
		ProductVersion fileVersion `json:"ProductVersion"`
	} `json:"FixedFileInfo"`
	StringFileInfo struct {
		CompanyName      string `json:"CompanyName"`
		ProductName      string `json:"ProductName"`
		FileDescription  string `json:"FileDescription"`
		InternalName     string `json:"InternalName"`
		OriginalFilename string `json:"OriginalFilename"`
		LegalCopyright   string `json:"LegalCopyright"`
		FileVersion      string `json:"FileVersion"`
		ProductVersion   string `json:"ProductVersion"`
	} `json:"StringFileInfo"`
	VarFileInfo struct {
		Translation struct {
			LangID    int `json:"LangID"`
			CharsetID int `json:"CharsetID"`
		} `json:"Translation"`
	} `json:"VarFileInfo"`
	IconPath string `json:"IconPath"`
}

func parseVersion(s string) (major, minor, patch int, err error) {
	// Strip snapshot/pre-release suffix (e.g. "0.1.0-SNAPSHOT-799e0b6" → "0.1.0")
	clean := strings.SplitN(s, "-", 2)[0]

	parts := strings.Split(clean, ".")
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("version must be in MAJOR.MINOR.PATCH form, got %q", s)
	}
	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid major version %q: %v", parts[0], err)
	}
	minor, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid minor version %q: %v", parts[1], err)
	}
	patch, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid patch version %q: %v", parts[2], err)
	}
	return major, minor, patch, nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: gen-versioninfo <version>  (e.g. 0.1.0)")
		os.Exit(1)
	}
	version := os.Args[1]

	major, minor, patch, err := parseVersion(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-versioninfo: %v\n", err)
		os.Exit(1)
	}

	raw, err := os.ReadFile("versioninfo-meta.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-versioninfo: cannot read versioninfo-meta.json: %v\n", err)
		os.Exit(1)
	}
	var m meta
	if err := json.Unmarshal(raw, &m); err != nil {
		fmt.Fprintf(os.Stderr, "gen-versioninfo: cannot parse versioninfo-meta.json: %v\n", err)
		os.Exit(1)
	}

	fv := fileVersion{Major: major, Minor: minor, Patch: patch, Build: 0}

	var vi versionInfo
	vi.FixedFileInfo.FileVersion = fv
	vi.FixedFileInfo.ProductVersion = fv
	vi.StringFileInfo.CompanyName = m.CompanyName
	vi.StringFileInfo.ProductName = m.ProductName
	vi.StringFileInfo.FileDescription = m.FileDescription
	vi.StringFileInfo.InternalName = m.InternalName
	vi.StringFileInfo.OriginalFilename = m.OriginalFilename
	vi.StringFileInfo.LegalCopyright = m.LegalCopyright
	vi.StringFileInfo.FileVersion = version
	vi.StringFileInfo.ProductVersion = version
	vi.VarFileInfo.Translation.LangID = 1033
	vi.VarFileInfo.Translation.CharsetID = 1200
	vi.IconPath = m.IconPath

	out, err := json.MarshalIndent(vi, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gen-versioninfo: cannot marshal JSON: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("cmd/auspex/versioninfo.json", out, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gen-versioninfo: cannot write cmd/auspex/versioninfo.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("gen-versioninfo: wrote cmd/auspex/versioninfo.json")
}
