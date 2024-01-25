package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/kevinburke/ssh_config"
	"golang.org/x/crypto/ssh"
)

var (
	host      = flag.String("host", "", "The host entry in ssh_config")
	logFile   = flag.String("logfile", "", "The full path to the remote log")
	localFile = flag.String("localfile", "", "The local filename to save the log as")
	clear     = flag.Bool("clear", false, "Whether to clear the remote file")
)

// this is primarily to just consolidate function args
type SSHConnection struct {
	Host   string
	Port   string
	Config *ssh.ClientConfig
}

func main() {
	flag.Parse()
	if len(*host) == 0 || len(*logFile) == 0 || len(*localFile) == 0 {
		fmt.Println("Usage:", os.Args[0], "<args>")
		flag.PrintDefaults()
		return
	}

	config := ssh.ClientConfig{
		User:            ssh_config.Get(*host, "user"),
		Auth:            privateKeyAuthMethod(ssh_config.Get(*host, "identityfile")),
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	host := SSHConnection{
		Host:   ssh_config.Get(*host, "hostname"),
		Port:   ssh_config.Get(*host, "port"),
		Config: &config,
	}

	err := fetchLog(*logFile, *localFile, host)
	if err != nil {
		log.Println(err)
	}

	if *clear {
		cmd := "sudo echo \"\" > " + *logFile
		_, err := runCommand(cmd, host)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

// Read the private key from disk, parse it and return an appropriate AuthMethod
func privateKeyAuthMethod(keyfile string) []ssh.AuthMethod {
	if strings.HasPrefix(keyfile, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil
		}
		keyfile = strings.ReplaceAll(keyfile, "~", home)
	}
	data, err := os.ReadFile(keyfile)
	if err != nil {
		log.Println(err)
		return nil
	}
	privkey, err := ssh.ParsePrivateKey(data)
	if err != nil {
		log.Println(err)
		return nil
	}
	return []ssh.AuthMethod{ssh.PublicKeys(privkey)}
}

// runCommand will make an SSH connection, issue a command on the remote
// system and return the bytes of the output.
//
// Commands that generate no output will return a nil slice of bytes.
func runCommand(cmd string, host SSHConnection) ([]byte, error) {
	conn, err := ssh.Dial("tcp", host.Host+":"+host.Port, host.Config)
	if err != nil {
		return nil, err
	}
	session, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()

	r, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := session.Start(cmd); err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)

	return buf.Bytes(), nil
}

// fetchLog will simply "cat <filename>" on the remote host while redirecting
// the output, which is then written to a local file.
//
// Local files are overwritten each time (not appending)
func fetchLog(remoteFile string, localFile string, host SSHConnection) error {
	output, err := runCommand("sudo truncate -s 0 "+remoteFile, host)
	if err != nil {
		return err
	}

	err = os.WriteFile(localFile, output, 0644)
	if err != nil {
		return err
	}
	return nil
}
