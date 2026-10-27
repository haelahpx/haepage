package discovery

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DockerClient talks to the Docker Engine API over its socket (or a TCP
// socket proxy) using read-only calls. No SDK: three endpoints are enough.
type DockerClient struct {
	http *http.Client
	base string
}

// NewDockerClient accepts "unix:///var/run/docker.sock" or
// "tcp://host:2375".
func NewDockerClient(host string) (*DockerClient, error) {
	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("docker host %q: %w", host, err)
	}
	transport := &http.Transport{MaxIdleConns: 2, IdleConnTimeout: 30 * time.Second}
	base := ""
	switch u.Scheme {
	case "unix":
		path := u.Path
		transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		}
		base = "http://docker"
	case "tcp", "http":
		base = "http://" + u.Host
	default:
		return nil, fmt.Errorf("docker host %q: use unix:// or tcp://", host)
	}
	return &DockerClient{http: &http.Client{Transport: transport}, base: base}, nil
}

// Container is the subset of the container list Haepage uses.
type Container struct {
	ID     string            `json:"Id"`
	Names  []string          `json:"Names"`
	Image  string            `json:"Image"`
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
	Ports  []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

// Name is the container name without Docker's leading slash.
func (c Container) Name() string {
	if len(c.Names) == 0 {
		return c.ID[:min(12, len(c.ID))]
	}
	return strings.TrimPrefix(c.Names[0], "/")
}

func (d *DockerClient) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.base+path, nil)
	if err != nil {
		return nil, err
	}
	res, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("docker %s: %s", path, res.Status)
	}
	return res, nil
}

// Containers lists all containers, running or not.
func (d *DockerClient) Containers(ctx context.Context) ([]Container, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := d.get(ctx, "/containers/json?all=1")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var list []Container
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("docker containers: %w", err)
	}
	return list, nil
}

// PortBindings returns a container's configured port bindings. Docker
// drops a stopped container's ports from the list, but its configuration
// keeps them, so a stopped service can stay on the page as "down".
func (d *DockerClient) PortBindings(ctx context.Context, id string) (map[string][]struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res, err := d.get(ctx, "/containers/"+url.PathEscape(id)+"/json")
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var info struct {
		HostConfig struct {
			PortBindings map[string][]struct {
				HostIP   string `json:"HostIp"`
				HostPort string `json:"HostPort"`
			} `json:"PortBindings"`
		} `json:"HostConfig"`
	}
	if err := json.NewDecoder(res.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("docker inspect: %w", err)
	}
	return info.HostConfig.PortBindings, nil
}

// withConfiguredPorts fills in the ports of a stopped container from its
// configuration.
func (d *DockerClient) withConfiguredPorts(ctx context.Context, c Container) Container {
	if c.State == "running" || len(c.Ports) > 0 {
		return c
	}
	bindings, err := d.PortBindings(ctx, c.ID)
	if err != nil {
		return c
	}
	for key, list := range bindings {
		portStr, proto, _ := strings.Cut(key, "/")
		private, err := strconv.Atoi(portStr)
		if err != nil || (proto != "" && proto != "tcp") {
			continue
		}
		for _, b := range list {
			public, err := strconv.Atoi(b.HostPort)
			if err != nil || public == 0 {
				continue
			}
			ip := b.HostIP
			if ip == "" {
				ip = "0.0.0.0"
			}
			c.Ports = append(c.Ports, struct {
				IP          string `json:"IP"`
				PrivatePort int    `json:"PrivatePort"`
				PublicPort  int    `json:"PublicPort"`
				Type        string `json:"Type"`
			}{ip, private, public, "tcp"})
		}
	}
	return c
}

// Events calls onEvent for each container lifecycle event until ctx ends
// or the stream breaks.
func (d *DockerClient) Events(ctx context.Context, onEvent func(action string)) error {
	filters := url.QueryEscape(`{"type":["container"],"event":["create","start","stop","die","destroy","rename","pause","unpause","health_status"]}`)
	res, err := d.get(ctx, "/events?filters="+filters)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	dec := json.NewDecoder(bufio.NewReader(res.Body))
	for {
		var e struct {
			Action string `json:"Action"`
		}
		if err := dec.Decode(&e); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("docker events: %w", err)
		}
		onEvent(e.Action)
	}
}
