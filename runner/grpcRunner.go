// Copyright 2018 ThoughtWorks, Inc.

// This file is part of Gauge.

// Gauge is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// Gauge is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with Gauge.  If not, see <http://www.gnu.org/licenses/>.

package runner

import (
	"context"
	"fmt"
	"io"
	"net"
	"os/exec"
	"strings"

	gm "github.com/getgauge/gauge/gauge_messages"
	"github.com/getgauge/gauge/manifest"
	"google.golang.org/grpc"
)

const (
	portPrefix = "Listening on port:"
	host       = "127.0.0.1"
)

// GrpcRunner handles grpc messages.
type GrpcRunner struct {
	cmd    *exec.Cmd
	conn   *grpc.ClientConn
	Client gm.LspServiceClient
}

// ExecuteMessageWithTimeout process reuqest and give back the response
func (r *GrpcRunner) ExecuteMessageWithTimeout(message *gm.Message) (*gm.Message, error) {
	switch message.MessageType {
	case gm.Message_CacheFileRequest:
		r.Client.CacheFile(context.Background(), message.CacheFileRequest)
		return &gm.Message{}, nil
	case gm.Message_StepNamesRequest:
		response, err := r.Client.GetStepNames(context.Background(), message.StepNamesRequest)
		return &gm.Message{StepNamesResponse: response}, err
	case gm.Message_StepPositionsRequest:
		response, err := r.Client.GetStepPositions(context.Background(), message.StepPositionsRequest)
		return &gm.Message{StepPositionsResponse: response}, err
	case gm.Message_ImplementationFileListRequest:
		response, err := r.Client.GetImplementationFiles(context.Background(), &gm.Empty{})
		return &gm.Message{ImplementationFileListResponse: response}, err
	case gm.Message_StubImplementationCodeRequest:
		response, err := r.Client.ImplementStub(context.Background(), message.StubImplementationCodeRequest)
		return &gm.Message{FileDiff: response}, err
	case gm.Message_StepValidateRequest:
		response, err := r.Client.ValidateStep(context.Background(), message.StepValidateRequest)
		return &gm.Message{MessageType: gm.Message_StepValidateResponse, StepValidateResponse: response}, err
	case gm.Message_RefactorRequest:
		response, err := r.Client.Refactor(context.Background(), message.RefactorRequest)
		return &gm.Message{MessageType: gm.Message_RefactorResponse, RefactorResponse: response}, err
	case gm.Message_StepNameRequest:
		response, err := r.Client.GetStepName(context.Background(), message.StepNameRequest)
		return &gm.Message{MessageType: gm.Message_StepNameResponse, StepNameResponse: response}, err
	case gm.Message_ImplementationFileGlobPatternRequest:
		response, err := r.Client.GetGlobPatterns(context.Background(), &gm.Empty{})
		return &gm.Message{MessageType: gm.Message_ImplementationFileGlobPatternRequest, ImplementationFileGlobPatternResponse: response}, err
	default:
		return nil, nil
	}
}

func (r *GrpcRunner) ExecuteAndGetStatus(m *gm.Message) *gm.ProtoExecutionResult {
	return nil
}
func (r *GrpcRunner) IsProcessRunning() bool {
	return false
}

// Kill closes the grpc connection and kills the process
func (r *GrpcRunner) Kill() error {
	r.Client.KillProcess(context.Background(), &gm.KillProcessRequest{})
	if err := r.conn.Close(); err != nil {
		return err
	}
	// TODO: wait for process to exit or kill forcefully after runner kill timeout
	if err := r.cmd.Process.Kill(); err != nil {
		return err
	}
	return nil
}

func (r *GrpcRunner) Connection() net.Conn {
	return nil
}

func (r *GrpcRunner) IsMultithreaded() bool {
	return false
}

func (r *GrpcRunner) Pid() int {
	return 0
}

type customWriter struct {
	file io.Writer
	port chan string
}

func (w customWriter) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), portPrefix) {
		w.port <- strings.TrimSuffix(strings.Split(string(p), portPrefix)[1], "\n")
	}
	return w.file.Write(p)
}

// ConnectToGrpcRunner makes a connection with grpc server
func ConnectToGrpcRunner(manifest *manifest.Manifest, outFile io.Writer) (*GrpcRunner, error) {
	portChan := make(chan string)
	cmd, _, err := runRunnerCommand(manifest, "0", false, customWriter{file: outFile, port: portChan})
	if err != nil {
		return nil, err
	}
	port := <-portChan
	close(portChan)
	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return nil, err
	}
	return &GrpcRunner{Client: gm.NewLspServiceClient(conn), cmd: cmd, conn: conn}, nil
}
