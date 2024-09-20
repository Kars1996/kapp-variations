package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
)

type Completed string

const (
	Confirm Completed = "confirm"
	Input   Completed = "input"
)

type KPrompts struct {
	colors map[string]string
}

func NewKPrompts() *KPrompts {
	kp := &KPrompts{}
	kp.fixColors()
	kp.colors = map[string]string{
		"cyan":  "\033[0;96m",
		"green": "\033[0;92m",
		"red":   "\033[0;91m",
		"white": "\033[0;97m",
		"grey":  "\033[1;30m",
	}
	return kp
}

func (kp *KPrompts) fixColors() {
	if !term.IsTerminal(int(syscall.Stdout)) {
		for key := range kp.colors {
			kp.colors[key] = ""
		}
	} else {
		if runtime := strings.ToLower(os.Getenv("OS")); strings.Contains(runtime, "windows") {
			kernel32 := syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")
			kernel32.Call(uintptr(syscall.Stdout), 7)
		}
	}
}

func (kp *KPrompts) Print(text string) {
	fmt.Printf("%s%s ", kp.colors["white"], text)
}

func (kp *KPrompts) FinalPrint(question, answer string, password bool) {
	fmt.Printf("%s√%s %s %s»%s %s\n", kp.colors["green"], kp.colors["white"], question, kp.colors["grey"], kp.colors["white"], answer)
}

func (kp *KPrompts) BetterInput(text string) string {
	kp.Print(fmt.Sprintf("%s? %s%s", kp.colors["cyan"], kp.colors["white"], text))
	var userInput string
	fmt.Scanln(&userInput)
	fmt.Printf("\033[F\033[K") // Clears previous input
	return userInput
}

func (kp *KPrompts) Prompt(option Completed, message string, validate func(string) bool, keep bool) string {
	switch option {
	case Input:
		for {
			userInput := kp.BetterInput(message)
			if validate != nil {
				if validate(userInput) {
					if keep {
						kp.FinalPrint(message, userInput, false)
					}
					return userInput
				} else {
					kp.Print(fmt.Sprintf("%s× Invalid input. Try again.%s", kp.colors["red"], kp.colors["white"]))
				}
			} else {
				if keep {
					kp.FinalPrint(message, userInput, false)
				}
				return userInput
			}
		}
	case Confirm:
		for {
			userInput := strings.ToLower(kp.BetterInput(fmt.Sprintf("%s %s(y/n)", message, kp.colors["grey"])))
			if userInput == "y" || userInput == "n" {
				if keep {
					kp.FinalPrint(message, userInput, false)
				}
				return userInput
			} else {
				kp.Print(fmt.Sprintf("%s× Please answer with 'y' or 'n'.%s", kp.colors["red"], kp.colors["white"]))
			}
		}
	default:
		panic("Invalid option")
	}
}

type CreateKapp struct {
	user    string
	branch  string
	urls    []string
	colors  map[string]string
	prompt  *KPrompts
	foundPath string
}

func NewCreateKapp(user, branch string) *CreateKapp {
	ck := &CreateKapp{
		user:   user,
		branch: branch,
		urls:   []string{"template", "apitemplate", "DJS14Template"},
		colors: map[string]string{
			"cyan":  "\033[0;96m",
			"green": "\033[0;92m",
			"red":   "\033[0;91m",
			"white": "\033[0;97m",
		},
		prompt: NewKPrompts(),
	}
	return ck
}

func (ck *CreateKapp) SetPath(path string) string {
	if path == "." {
		ck.foundPath, _ = os.Getwd()
	} else if _, err := os.Stat(path); os.IsNotExist(err) {
		os.Mkdir(path, os.ModePerm)
		ck.foundPath, _ = filepath.Abs(path)
	} else {
		ck.foundPath, _ = filepath.Abs(path)
	}
	return ck.foundPath
}

func (ck *CreateKapp) Download(url string) {
	downloadURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip", ck.user, url, ck.branch)
	ck.prompt.Print(fmt.Sprintf("%s∂ Downloading template %s...%s", ck.colors["cyan"], url, ck.colors["white"]))

	resp, err := http.Get(downloadURL)
	if err != nil || resp.StatusCode != 200 {
		ck.prompt.Print(fmt.Sprintf("%sFailed to download: %d%s", ck.colors["red"], resp.StatusCode, ck.colors["white"]))
		time.Sleep(2 * time.Second)
		return
	}
	defer resp.Body.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		ck.prompt.Print(fmt.Sprintf("%sError occurred: %s%s", ck.colors["red"], err.Error(), ck.colors["white"]))
		time.Sleep(2 * time.Second)
		return
	}

	ck.prompt.Print("Extracting...")
	reader := bytes.NewReader(buf.Bytes())
	zipReader, err := zip.NewReader(reader, int64(buf.Len()))
	if err != nil {
		ck.prompt.Print(fmt.Sprintf("%sError occurred: %s%s", ck.colors["red"], err.Error(), ck.colors["white"]))
		time.Sleep(2 * time.Second)
		return
	}

	for _, file := range zipReader.File {
		fpath := filepath.Join(ck.foundPath, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
		} else {
			outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
			if err != nil {
				ck.prompt.Print(fmt.Sprintf("%sError occurred: %s%s", ck.colors["red"], err.Error(), ck.colors["white"]))
				time.Sleep(2 * time.Second)
				return
			}
			rc, err := file.Open()
			if err != nil {
				outFile.Close()
				ck.prompt.Print(fmt.Sprintf("%sError occurred: %s%s", ck.colors["red"], err.Error(), ck.colors["white"]))
				time.Sleep(2 * time.Second)
				return
			}
			io.Copy(outFile, rc)
			outFile.Close()
			rc.Close()
		}
	}

	ck.prompt.Print("Download and extraction complete!")
}

func (ck *CreateKapp) Run() {
	folder := ck.prompt.Prompt(Input, "Setup the project in (specify folder)...?", func(value string) bool {
		return len(value) > 0
	}, true)
	ck.SetPath(folder)

	scaffold := ck.prompt.Prompt(Input, "What scaffold do you want to start with?", nil, true)
	if scaffold != "" {
		if contains(ck.urls, scaffold) {
			ck.Download(scaffold)
		} else {
			ck.Download(ck.urls[0])
		}
	}
	ck.prompt.Print("Successfully set up project :D")
}

func contains(s []string, str string) bool {
	for _, v := range s {
		if v == str {
			return true
		}
	}
	return false
}

func main() {
	ck := NewCreateKapp("kars1996", "master")
	ck.Run()
}
