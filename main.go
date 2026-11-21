package rotacionador

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

const (
	socksAddress   = "127.0.0.1:9050"
	controlAddress = "127.0.0.1:9051"
)

func NewTorClient() (*http.Client, *http.Transport, error) {

	dialer, err := proxy.SOCKS5(
		"tcp",
		socksAddress,
		nil,
		&net.Dialer{Timeout: 30 * time.Second},
	)

	if err != nil {
		return nil, nil, err
	}

	transport := &http.Transport{
		DialContext: func(
			ctx context.Context,
			network string,
			addr string,
		) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	return client, transport, nil
}

func readReply(reader *bufio.Reader) (string, error) {
	var response strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return response.String(), err
		}

		response.WriteString(line)

		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}

	result := response.String()

	if !strings.HasPrefix(result, "250 ") {
		return result, fmt.Errorf("Tor retornou: %s", result)
	}

	return result, nil
}

func RotateIP(transport *http.Transport) error {

	conn, err := net.DialTimeout(
		"tcp",
		controlAddress,
		5*time.Second,
	)

	if err != nil {
		return err
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	sendCommand := func(command string) error {
		if _, err := writer.WriteString(command + "\r\n"); err != nil {
			return err
		}

		if err := writer.Flush(); err != nil {
			return err
		}

		_, err := readReply(reader)
		return err
	}

	// Autenticação sem senha
	if err := sendCommand("AUTHENTICATE"); err != nil {
		return err
	}

	// Solicita novos circuitos
	if err := sendCommand("SIGNAL NEWNYM"); err != nil {
		return err
	}

	// Fecha conexões ociosas para evitar reutilização
	transport.CloseIdleConnections()

	return nil
}

// Consulta o IP de saída
func CheckIP(client *http.Client) error {

	resp, err := client.Get("https://api.ipify.org")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("IP de saída:", string(ip))

	return nil
}

func execRotacionador() {

	client, transport, err := NewTorClient()
	if err != nil {
		panic(err)
	}
	defer transport.CloseIdleConnections()

	// Primeiro IP
	fmt.Println("IP inicial:")
	if err := CheckIP(client); err != nil {
		panic(err)
	}

	// Solicita novos circuitos
	fmt.Println("\nSolicitando novos circuitos...")

	if err := RotateIP(transport); err != nil {
		panic(err)
	}

	fmt.Println("Solicitação aceita!")

	// Aguarda um pouco antes da próxima conexão
	time.Sleep(5 * time.Second)

	// Segundo IP
	fmt.Println("\nIP após solicitação:")
	if err := CheckIP(client); err != nil {
		panic(err)
	}
}
