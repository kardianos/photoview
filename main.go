package main

import (
	"flag"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"bitbucket.org/kardianos/osext"
	"github.com/BurntSushi/toml"
	"github.com/kardianos/service"
)

var config = struct {
	ListenOn    string
	FileRoot    string
	TrashFolder string
	CacheFolder string
	ThumbRes    int
	SmallRes    int

	Debug bool
}{
	ListenOn:    ":8080",
	FileRoot:    "/data/store/Pictures",
	TrashFolder: "/data/store/Pictures/Trash",
	CacheFolder: "/data/store/scratch/cache",
	ThumbRes:    400,
	SmallRes:    1200,

	Debug: false,
}

const (
	conifgFile = "photoview.toml"

	photosUrl   = "/photos"
	downloadUrl = "/download"

	tmplDir  = "dir.tmpl"
	badThumb = "bad_thumb.jpg"
)

var logger service.Logger

// Program structures.
//  Define Start and Stop methods.
type program struct {
	exit     chan struct{}
	execDir  string
	cacheDir string
	listener net.Listener

	T *template.Template
}

func (p *program) Start(s service.Service) error {
	runtime.GOMAXPROCS(runtime.NumCPU())
	p.exit = make(chan struct{})
	var err error

	p.execDir, err = osext.ExecutableFolder()
	if err != nil {
		return err
	}
	err = p.loadOrWriteConifg()
	if err != nil {
		return err
	}

	l, err := net.Listen("tcp", config.ListenOn)
	if err != nil {
		return err
	}
	p.listener = l

	if filepath.IsAbs(config.CacheFolder) {
		p.cacheDir = config.CacheFolder
	} else {
		p.cacheDir = filepath.Join(p.execDir, config.CacheFolder)
	}
	err = p.loadTemplate()
	if err != nil {
		return err
	}

	// Start should not block. Do the actual work async.
	logger.Infof("Starting. Listen to %s", config.ListenOn)
	go p.run(l)
	return nil
}

func (p *program) loadOrWriteConifg() error {
	configPath := filepath.Join(p.execDir, conifgFile)
	_, err := toml.DecodeFile(configPath, &config)
	if os.IsNotExist(err) {
		f, err := os.Create(configPath)
		if err != nil {
			return err
		}
		coder := toml.NewEncoder(f)
		err = coder.Encode(config)
		f.Close()
		if err != nil {
			return err
		}
	}
	return err
}
func (p *program) loadTemplate() error {
	var err error
	p.T, err = template.New("").Funcs(tmplFunc).ParseGlob(filepath.Join(p.execDir, "template", "*.tmpl"))
	return err
}

func (p *program) run(listener net.Listener) {
	http.Handle(photosUrl+"/", http.StripPrefix(photosUrl, p))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, photosUrl+"/", http.StatusMovedPermanently)
	})
	fs := http.FileServer(http.Dir(filepath.Join(p.execDir, "lib")))
	http.Handle("/lib/", http.StripPrefix("/lib", fs))
	http.Handle("/api/", http.StripPrefix("/api", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeAPI(w, r)
	})))

	err := http.Serve(listener, nil)
	if err != nil {
		logger.Error(err)
		os.Exit(1)
	}
}
func (p *program) Stop(s service.Service) error {
	close(p.exit)
	_ = p.listener.Close()
	return nil
}

func (p *program) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	atIndex := strings.IndexRune(r.URL.Path, '@')
	urlPath := r.URL.Path
	resolutionString := ""
	if atIndex >= 0 {
		urlPath = r.URL.Path[:atIndex]
		resolutionString = r.URL.Path[atIndex+1:]
	}

	fullPath := filepath.Join(config.FileRoot, urlPath)
	fi, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.Error(err)
		return
	}
	if fi.IsDir() {
		if config.Debug {
			err := p.loadTemplate()
			if err != nil {
				logger.Error(err)
				http.Error(w, err.Error(), 500)
				return
			}
		}
		dt, err := NewDirTmpl(fullPath, urlPath)
		// TODO: buffer template.
		err = p.T.ExecuteTemplate(w, "dir.tmpl", dt)
		if err != nil {
			logger.Error("Template dir.tmpl", err)
			return
		}
		return
	}

	err = p.serveFile(w, r, fullPath, urlPath, resolutionString)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		logger.Error(err)
		return
	}
}
func (p *program) ServeAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		p.apiPOST(w, r)
	case "GET":
		p.apiGET(w, r)
	default:
		http.Error(w, "not found", 404)
	}
}
func (p *program) apiPOST(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Parse form error", 500)
		return
	}
	list := r.Form["list"]
	reqPath := r.Form.Get("path")
	if len(reqPath) > len(photosUrl) {
		reqPath = reqPath[len(photosUrl):]
	}

	switch r.URL.Path {
	case "/delete":
		for _, item := range list {
			err = os.Rename(
				filepath.Join(config.FileRoot, reqPath, item),
				filepath.Join(config.TrashFolder, item),
			)
			if err != nil {
				break
			}
		}
	case "/refresh-cache":
		err = p.refreshCache(reqPath, list)
	case "/rot-left":
		err = p.editImage(RotLeft, reqPath, list)
	case "/rot-right":
		err = p.editImage(RotRight, reqPath, list)
	case "/rot-flip":
		err = p.editImage(RotFlip, reqPath, list)
	case "/download-full":
		var key string
		key, err = p.downloadAssignKey(reqPath, list, false)
		if err == nil {
			w.Write([]byte("/api" + downloadUrl + "?key=" + key))
		}
	case "/download-small":
		var key string
		key, err = p.downloadAssignKey(reqPath, list, true)
		if err == nil {
			w.Write([]byte("/api" + downloadUrl + "?key=" + key))
		}
	default:
		http.Error(w, "Unknown api call", 404)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
func (p *program) apiGET(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.URL.Path {
	case downloadUrl:
		key := r.URL.Query().Get("key")
		err = p.downloadWriteKey(key, w)
	default:
		http.Error(w, "Unknown api call", 404)
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		logger.Error("Error downloading key: ", err)
	}
}

func main() {
	svcFlag := flag.String("service", "", "Control the system service.")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "photoview",
		DisplayName: "Photo Viewer",
		Description: "Photo viewer.",
	}

	prg := &program{}
	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}
	errs := make(chan error, 5)
	logger, err = s.Logger(errs)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			err := <-errs
			if err != nil {
				log.Print(err)
			}
		}
	}()

	if len(*svcFlag) != 0 {
		err := service.Control(s, *svcFlag)
		if err != nil {
			log.Printf("Valid actions: %q\n", service.ControlAction)
			log.Fatal(err)
		}
		return
	}
	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}
