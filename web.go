package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	utime "github.com/chennqqi/goutils/time"

	"github.com/chennqqi/goutils/persistlist"
	"github.com/gin-gonic/gin"
	mutils "github.com/malice-plugins/go-plugin-utils/utils"
	"github.com/satori/uuid"
)

const YGOBUSTERJOBLIST = "__subname_dns_buster_checkList"

var (
	resultExp = regexp.MustCompile(`(?m)^(.{1,10})\b\s+\(([^\)]+)\)`)
)

type CommonResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type CR CommonResponse

type Web struct {
	to utime.Duration

	callback string
	potDir   string

	tmpDir   string
	indexDir string

	scanQuit chan struct{}
	server   *http.Server
	cancel   context.CancelFunc
	list     persistlist.PersistList
}

type weakPassJob struct {
	Cb   string         `json:"cb"`
	Name string         `json:"name"`
	Pot  string         `json:"pot"`
	Tid  string         `json:"tid"`
	To   utime.Duration `json:"to"`
}

func jtrSimple(file, potName, tid string, to time.Duration) (string, error) {
	fmt.Println("start simple crack")
	ctx, cancel := context.WithTimeout(context.TODO(), to)
	defer cancel()

	/*
		docker run -it -v `pwd`/yourfiletocrack:/crackme.txt adamoss/john-the-ripper /crackme.txt
	*/

	type RcdList struct {
		User  string `json:"user"`
		Pass  string `json:"pass"`
		Crypt string `json:"crypt"`
	}
	var ret struct {
		Tid     string    `json:"tid"`
		Status  int       `json:"status"`
		List    []RcdList `json:"list"`
		Message string    `json:"message"`
	}

	defer os.Remove(potName)
	defer os.RemoveAll("/home/malice/.john")
	r, err := mutils.RunCommand(ctx, "/usr/sbin/john", "--pot="+potName, file)
	if err != nil {
		ret.Status = 500
		ret.Message = err.Error()
		ret.Tid = tid
		fmt.Println("run ERROR:", err)
	} else {
		ret.Status = 200
		ret.Message = "OK"
		ret.Tid = tid

		txt, err := ioutil.ReadFile(file)
		if err != nil {
			ret.Status = 501
			ret.Message = err.Error()
			ret.Tid = tid
		} else {
			lines := bytes.SplitN(txt, []byte("\n"), -1)
			cryptMap := make(map[string]string)
			for i := 0; i < len(lines); i++ {
				item := bytes.Split(lines[i], []byte(":"))
				if len(item) > 2 {
					cryptMap[string(item[0])] = string(item[1])
				}
			}

			results := resultExp.FindAllStringSubmatch(r, -1)
			var listSize int
			var listIdx int
			if len(cryptMap) < len(results) {
				listSize = len(cryptMap)
			} else {
				listSize = len(results)
			}
			ret.List = make([]RcdList, listSize)

			for idx := 0; idx < len(results) && listIdx < listSize; idx++ {
				pass := results[idx][1]
				user := results[idx][2]
				crypt, exist := cryptMap[user]
				if !exist {
					fmt.Println("NOT FOUND", pass)
					continue
				}
				ret.List[listIdx].User = user
				ret.List[listIdx].Pass = pass
				ret.List[listIdx].Crypt = crypt
				fmt.Println(ret.List[listIdx])
				listIdx++
			}
			if listIdx != listSize {
				ret.List = ret.List[:listIdx]
			}
		}
	}
	txt, _ := json.Marshal(ret)
	return string(txt), nil
}

func jtrWordList(dir string, to time.Duration) (string, error) {
	fmt.Println("start scan ", dir)
	ctx, cancel := context.WithTimeout(context.TODO(), to)
	defer cancel()
	return mutils.RunCommand(ctx, "/usr/sbin/john", "wordlist=", dir)
}

func (s *Web) scanRoute(ctx context.Context) {
	ticker := time.NewTicker(time.Second / 2)
	defer ticker.Stop()
	list := s.list

__FOR_LOOP:
	for {
		select {
		case <-ticker.C:
			for {
				var task weakPassJob
				err := list.Pop(&task)
				if err == persistlist.ErrNil {
					break
				}
				if err != nil {
					fmt.Println("[scanRoute] POP ERROR:", err)
					continue
				}
				r, _ := jtrSimple(task.Name, task.Pot, task.Tid, time.Duration(task.To))
				s.doCallback(task.Cb, r)
				os.Remove(task.Name)
			}
		case <-ctx.Done():
			break __FOR_LOOP
		}
	}
	close(s.scanQuit)
}

func (s *Web) simple(c *gin.Context) {
	var err error
	var exist bool
	to := s.to
	timeout, ok := c.GetQuery("timeout")
	if ok {
		tto, err := time.ParseDuration(timeout)
		if err == nil {
			to = utime.Duration(tto)
		}
	}

	upf, err := c.FormFile("filename")
	if err != nil {
		c.JSON(400,
			CR{
				Message: fmt.Sprintf("get form err: %s", err.Error()),
				Status:  1,
			})
		return
	}
	src, err := upf.Open()
	if err != nil {
		c.JSON(400, CR{
			Message: fmt.Sprintf("open form err:  %s", err.Error()),
			Status:  1,
		})
		return
	}
	defer src.Close()
	f, err := ioutil.TempFile(s.tmpDir, "shadow_")
	if err != nil {
		c.JSON(500, CR{
			Message: fmt.Sprintf("open new tmp file err:  %s", err.Error()),
			Status:  1,
		})
		return
	}
	io.Copy(f, src)
	f.Close()
	tid := uuid.Must(uuid.NewV4()).String()
	job := weakPassJob{
		To:   to,
		Tid:  tid,
		Pot:  filepath.Join(s.potDir, tid),
		Name: f.Name(),
	}
	job.Cb, exist = c.GetQuery("callback")
	if !exist {
		job.Cb = s.callback
		exist = true
	}

	{
		var ret struct {
			CR
			TID     string `json:"tid"`
			Pending int64  `json:"pending"`
		}
		ret.Status = 0
		ret.Message = "OK"
		ret.TID = job.Tid
		ret.Pending = 0

		list := s.list
		l, err := list.Push(job)
		ret.Pending = l
		if err != nil {
			ret.Message = err.Error()
			ret.Status = 1
			c.JSON(500, ret)
			return
		}
		c.JSON(200, ret)
	}
}

func (s *Web) version(c *gin.Context) {
	txt, _ := ioutil.ReadFile("/opt/ygobuster/VERSION")
	c.Data(200, "", txt)
}

func (s *Web) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	s.cancel()
	<-s.scanQuit
	return err
}

func (s *Web) queued(c *gin.Context) {
	list := s.list
	l, err := list.Len()
	if err != nil {
		c.String(400, "%v", err)
		return
	}
	c.String(200, "%d", l)
}

func (s *Web) flush(c *gin.Context) {
	list := s.list
	var count int
	var ret struct {
		CR        //common response
		Count int `json:"count"`
	}

	for {
		var task weakPassJob
		err := list.Pop(&task)
		if err == persistlist.ErrNil {
			break
		} else if err != nil {
			ret.Message = err.Error()
			ret.Status = 1
			ret.Count = count
			c.JSON(500, &ret)
		}
		//os.Remove(task.Name)

		count++
	}
	ret.Message = "OK"
	ret.Status = 0
	ret.Count = count
	c.JSON(200, &ret)
}

func (s *Web) Run(port int, ctx context.Context) error {
	scanctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.scanQuit = make(chan struct{})
	go s.scanRoute(scanctx)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.GET("/version", s.version)
	r.POST("/simple", s.simple)
	r.GET("/queued", s.queued)
	r.POST("/flush", s.flush)

	r.GET("/status", s.status)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}
	s.server = httpServer
	return httpServer.ListenAndServe()
}

func NewWeb(dataDir, indexDir, cb string, to time.Duration) (*Web, error) {
	var web Web

	list, err := persistlist.NewNodbList(indexDir, YGOBUSTERJOBLIST)
	if err != nil {
		return nil, err
	}
	web.list = list
	web.tmpDir = dataDir
	web.to = utime.Duration(to)
	web.callback = cb
	return &web, nil
}

func (s *Web) doCallback(cb string, r string) {
	if cb == "" {
		cb = s.callback
	}
	//TODO: user http client
	if cb != "" {
		go func(r string, cb string) {
			body := strings.NewReader(r)
			http.Post(cb, "application/json", body)
		}(r, cb)
	}
}
