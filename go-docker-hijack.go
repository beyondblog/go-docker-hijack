package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	dockerHost  = flag.String("docker_api", "http://127.0.0.1:2375", "Docker remote api")
	containerId = flag.String("id", "", "container id")
	exec        = flag.String("exec", "", "shell command")
	attach      = flag.Bool("attach", false, "attach container")
	ps          = flag.Bool("ps", false, "list container")
)

type ContainerResponse struct {
	items []Container
}

type ExecResponse struct {
	Id string
}

type Container struct {
	Id    string
	Names []string
}

func ListContainers() {
	resp, err := http.Get(*dockerHost + "/containers/json")
	if err != nil {
		log.Fatal(err)
	}

	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	items := make([]Container, 0)
	json.Unmarshal(body, &items)
	for _, container := range items {
		fmt.Printf("%s %s\r\n", container.Id[:12], container.Names[0][1:])
	}
}

func CreateExec(id string, cmd string) (string, error) {
	var jsonBody = strings.NewReader(`{
		"AttachStdin": true,
		"AttachStdout": true,
		"AttachStderr": true,
		"DetachKeys": "ctrl-p,ctrl-q",
		"Tty": true,
		"Cmd": [
		"/bin/bash"
		]
	}`)
	res, err := http.Post(*dockerHost+"/containers/"+id+"/exec", "application/json;charset=utf-8", jsonBody)

	if err != nil {
		return "", err
	}

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		return "", err
	}

	var result ExecResponse
	json.Unmarshal(body, &result)
	return result.Id, nil
}

func Connect(url *url.URL) {
	req, _ := http.NewRequest("POST", url.String(), strings.NewReader(
		`{
			"Detach": false,
			"Tty": true
		}`))
	dial, err := net.Dial("tcp", url.Host)
	if err != nil {
		log.Fatal(err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	clientconn := httputil.NewClientConn(dial, nil)
	clientconn.Do(req)
	defer clientconn.Close()

	rwc, br := clientconn.Hijack()
	defer rwc.Close()
	inputReader := bufio.NewReader(os.Stdin)
	go func() {
		for {
			input, _ := inputReader.ReadString('\n')
			rwc.Write([]byte(input))
		}
	}()

	for {
		buf := make([]byte, 100)
		_, err := br.Read(buf)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			log.Println("Read Err: " + err.Error())
		}
		fmt.Printf("%s", string(buf))
		time.Sleep(500)
		buf = nil
	}
	time.Sleep(1 * time.Second)
}

func main() {
	flag.Parse()
	if *ps {
		ListContainers()
	}

	if len(*containerId) > 0 {
		if *attach {
			attachUrl, _ := url.Parse(*dockerHost + "/containers/" + *containerId + "/attach?logs=1&stream=1&stdout=1&stdin=1")
			Connect(attachUrl)
			return
		}

		if len(*exec) > 0 {
			id, err := CreateExec(*containerId, *exec)
			if err != nil {
				log.Fatal(err)
			}
			execUrl, _ := url.Parse(*dockerHost + "/exec/" + id + "/start")
			Connect(execUrl)
		}
	}
}
