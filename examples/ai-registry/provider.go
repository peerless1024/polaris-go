// Tencent is pleased to support the open source community by making polaris-go available.
//
// Copyright (C) 2021 Tencent. All rights reserved.
//
// Licensed under the BSD 3-Clause License (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// https://opensource.org/licenses/BSD-3-Clause
//
// Unless required by applicable law or agreed to in writing, software distributed
// under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
// CONDITIONS OF ANY KIND, either express or implied. See the License for the
// specific language governing permissions and limitations under the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
)

type providerReady struct {
	RunID      string `json:"run_id"`
	Service    string `json:"service"`
	InstanceID string `json:"instance_id"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	URL        string `json:"url"`
	PID        int    `json:"pid"`
	Stopped    bool   `json:"stopped"`
}

type processExit struct {
	done chan struct{}
	err  error
}

func providerMetadata(service, endpoint, runID string) ([]byte, map[string]string, error) {
	card := map[string]interface{}{
		"name": service, "version": "1.0.0", "description": "Polaris Go real E2E agent",
		"supportedInterfaces": []map[string]string{{"url": endpoint + "/invoke", "protocolBinding": "HTTP+JSON", "protocolVersion": "1.0"}},
		"skills":              []map[string]string{{"id": "echo", "name": "echo", "description": "Return an observed request"}},
	}
	cardJSON, err := json.Marshal(card)
	if err != nil {
		return nil, nil, err
	}
	metadata, err := model.BuildAgentInstanceMetadata(model.AgentInstanceMetadata{
		AgentName: service, CardJSON: string(cardJSON), Transport: "HTTP+JSON", Path: "/invoke",
	})
	if err != nil {
		return nil, nil, err
	}
	metadata[ownerLabel], metadata[runLabel] = e2eOwner, runID
	return cardJSON, metadata, nil
}

func routeHost(address string) (string, error) {
	conn, err := net.Dial("udp", address)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || local.IP == nil {
		return "", fmt.Errorf("no route source IP")
	}
	return local.IP.String(), nil
}

func readSkillInstructions(installed *model.InstalledSkill) (string, error) {
	if installed == nil || installed.Dir == "" || installed.EntryFile == "" {
		return "", fmt.Errorf("installed Skill has no directory or entry file")
	}
	data, err := ioutil.ReadFile(filepath.Join(installed.Dir, filepath.FromSlash(installed.EntryFile)))
	return string(data), err
}

func serveProvider(ctx context.Context, o options) (result error) {
	cfg, err := loadConfiguration(o, o.skillName != "", false)
	if err != nil {
		return err
	}
	sdkCtx, err := api.InitContextByConfig(cfg)
	if err != nil {
		return err
	}
	defer sdkCtx.Destroy()
	instructions := "echo"
	if o.skillName != "" {
		installed, installErr := api.NewSkillAPIByContext(sdkCtx).Install(&api.SkillInstallRequest{SkillInstallRequest: model.SkillInstallRequest{
			Namespace: o.namespace, Name: o.skillName, Version: "1.0.0", CacheDir: filepath.Join(o.runDir, "cache"),
		}})
		if installErr != nil {
			return installErr
		}
		loaded, readErr := readSkillInstructions(installed)
		if readErr != nil {
			return readErr
		}
		instructions = loaded
	}
	host, err := routeHost(o.discover)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return err
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	endpoint := "http://" + net.JoinHostPort(host, fmt.Sprint(port))
	cardJSON, metadata, err := providerMetadata(o.service, endpoint, o.runID)
	if err != nil {
		return err
	}
	if err = api.NewAgentAPIByContext(sdkCtx).ValidateInstanceMetadata(metadata); err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/agent-card.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(cardJSON)
	})
	mux.HandleFunc("/invoke", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"service": o.service, "reply": "Echo: " + r.URL.Query().Get("message"), "instructions": instructions})
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		result = addError(result, server.Shutdown(shutdown))
	}()
	provider := api.NewProviderAPIByContext(sdkCtx)
	registration := &api.InstanceRegisterRequest{InstanceRegisterRequest: model.InstanceRegisterRequest{
		Namespace: o.namespace, Service: o.service, Host: host, Port: port, Metadata: metadata,
		ServiceToken: environment("POLARIS_E2E_SERVICE_TOKEN", "POLARIS_SERVICE_TOKEN"),
	}}
	registration.SetTTL(1)
	registered, err := provider.RegisterInstance(registration)
	if err != nil {
		return err
	}
	ready := providerReady{RunID: o.runID, Service: o.service, InstanceID: registered.InstanceID, Host: host, Port: port, URL: endpoint + "/invoke", PID: os.Getpid()}
	defer func() {
		request := &api.InstanceDeRegisterRequest{InstanceDeRegisterRequest: model.InstanceDeRegisterRequest{
			InstanceID: ready.InstanceID, Namespace: o.namespace, Service: o.service, Host: host, Port: port,
			ServiceToken: registration.ServiceToken,
		}}
		request.SetTimeout(o.timeout)
		result = addError(result, provider.Deregister(request))
		ready.Stopped = result == nil
		result = addError(result, writeJSON(o.readyPath, ready))
	}()
	if err = writeJSON(o.readyPath, ready); err != nil {
		return err
	}
	log.Printf("READY service=%s instance=%s host=%s port=%d", o.service, ready.InstanceID, host, port)
	select {
	case <-ctx.Done():
		return nil
	case err = <-serveDone:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func (s *suite) startProvider(ctx context.Context, label, skillName string) (providerReady, error) {
	service := s.serviceName(label)
	if existing, err := s.console.getService(ctx, s.opts.namespace, service); err != nil || existing != nil {
		if err != nil {
			return providerReady{}, err
		}
		return providerReady{}, fmt.Errorf("refuse to register into an existing run service")
	}
	if err := s.recordService(service); err != nil {
		return providerReady{}, err
	}
	path := filepath.Join(s.opts.runDir, label+"-ready.json")
	binary, err := os.Executable()
	if err != nil {
		return providerReady{}, err
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return providerReady{}, err
	}
	args := []string{"-mode=provider", "-discover=" + s.opts.discover, "-skill=" + s.opts.skill, "-config-address=" + s.opts.config,
		"-console=" + s.opts.console, "-namespace=" + s.opts.namespace, "-run-id=" + s.opts.runID,
		"-run-dir=" + filepath.Join(s.opts.runDir, label), "-service=" + service, "-ready=" + path, "-skill-name=" + skillName,
		"-timeout=" + s.opts.timeout.String()}
	if s.opts.configFile != "" {
		args = append(args, "-config-file="+s.opts.configFile)
	}
	if s.opts.debug {
		args = append(args, "-debug")
	}
	command := exec.Command(binary, args...)
	command.Dir = s.opts.runDir
	logFile, err := os.OpenFile(filepath.Join(s.opts.runDir, label+"-provider.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return providerReady{}, err
	}
	command.Stdout, command.Stderr = logFile, logFile
	if err = command.Start(); err != nil {
		logFile.Close()
		return providerReady{}, err
	}
	logFile.Close()
	process := processState{PID: command.Process.Pid, Binary: binary, ReadyPath: path, Service: service}
	s.state.Processes = append(s.state.Processes, process)
	if err = s.saveState(); err != nil {
		command.Process.Signal(syscall.SIGTERM)
		command.Wait()
		return providerReady{}, err
	}
	exit := &processExit{done: make(chan struct{})}
	go func() {
		exit.err = command.Wait()
		close(exit.done)
	}()
	s.processes[command.Process.Pid] = exit
	var ready providerReady
	err = eventually(ctx, s.opts.timeout+35*time.Second, func() error {
		select {
		case <-exit.done:
			return fmt.Errorf("provider exited before ready: %v", exit.err)
		default:
		}
		data, readErr := ioutil.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if err := json.Unmarshal(data, &ready); err != nil {
			return err
		}
		if ready.RunID != s.opts.runID || ready.Service != service || ready.PID != process.PID || ready.InstanceID == "" || ready.Stopped {
			return fmt.Errorf("invalid provider readiness")
		}
		return nil
	})
	if err != nil {
		select {
		case <-exit.done:
		default:
			err = addError(err, command.Process.Signal(syscall.SIGTERM))
			select {
			case <-exit.done:
			case <-time.After(20 * time.Second):
				err = addError(err, command.Process.Kill())
				<-exit.done
			}
		}
		return providerReady{}, addError(err, exit.err)
	}
	s.state.Instances = append(s.state.Instances, instanceState{ID: ready.InstanceID, Service: service})
	return ready, s.saveState()
}

func stopProvider(process processState, runID string) error {
	command, err := exec.Command("ps", "-p", fmt.Sprint(process.PID), "-o", "command=").Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return nil
		}
		return err
	}
	actual := strings.TrimSpace(string(command))
	if !strings.HasPrefix(actual, process.Binary+" ") || !hasCommandArgument(actual, "-mode=provider") ||
		!hasCommandArgument(actual, "-run-id="+runID) || !hasCommandArgument(actual, "-service="+process.Service) ||
		!hasCommandArgument(actual, "-ready="+process.ReadyPath) {
		return fmt.Errorf("refuse to signal reused or unowned PID %d", process.PID)
	}
	p, err := os.FindProcess(process.PID)
	if err != nil {
		return err
	}
	if err = p.Signal(syscall.SIGTERM); err != nil {
		if err == os.ErrProcessDone || err == syscall.ESRCH {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := ioutil.ReadFile(process.ReadyPath)
		if readErr == nil {
			var ready providerReady
			if json.Unmarshal(data, &ready) == nil && ready.Stopped && ready.RunID == runID && ready.PID == process.PID && ready.Service == process.Service {
				return nil
			}
		}
		if err = p.Signal(syscall.Signal(0)); err != nil {
			if err == os.ErrProcessDone || err == syscall.ESRCH {
				return nil
			}
			return err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("provider PID %d did not complete shutdown", process.PID)
}

func hasCommandArgument(command, argument string) bool {
	return strings.Contains(command, " "+argument+" ") || strings.HasSuffix(command, " "+argument)
}
