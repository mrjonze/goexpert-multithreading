package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"io"
	"log"
	"net/http"
	"time"
)

type BrasilApi struct {
	Cep          string `json:"cep"`
	State        string `json:"state"`
	City         string `json:"city"`
	Neighborhood string `json:"neighborhood"`
	Street       string `json:"street"`
	Service      string `json:"service"`
}

type ViaCep struct {
	Cep         string `json:"cep"`
	Logradouro  string `json:"logradouro"`
	Complemento string `json:"complemento"`
	Bairro      string `json:"bairro"`
	Localidade  string `json:"localidade"`
	Uf          string `json:"uf"`
	Ibge        string `json:"ibge"`
	Gia         string `json:"gia"`
	Ddd         string `json:"ddd"`
	Siafi       string `json:"siafi"`
}

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/cep/{cep}", SearchCepHandler)

	http.ListenAndServe(":8080", r)
}

func SearchCepHandler(w http.ResponseWriter, r *http.Request) {
	cepInput := chi.URLParam(r, "cep")
	if cepInput == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	cViaCep := make(chan string)
	cBrasilApi := make(chan string)

	addressTemplate := "Logradouro: %s, Bairro: %s, Cidade: %s, Estado: %s, CEP: %s"
	var response string
	// BrasilApi
	go func() {
		cepOutput, err := SearchCepBrasilApi(cepInput)

		if err != nil {
			log.Println(err)
			cBrasilApi <- "Erro ao buscar CEP pela Brasil Api"
			return
		}

		cBrasilApi <- fmt.Sprintf(addressTemplate, cepOutput.Street, cepOutput.Neighborhood, cepOutput.City, cepOutput.State, cepOutput.Cep)
	}()

	// ViaCep
	go func() {
		cepOutput, err := SearchCepViaCep(cepInput)

		if err != nil {
			log.Println(err)
			cViaCep <- "Erro ao buscar CEP pela ViaCep."
			return
		}

		cViaCep <- fmt.Sprintf(addressTemplate, cepOutput.Logradouro, cepOutput.Bairro, cepOutput.Localidade, cepOutput.Uf, cepOutput.Cep)
	}()

	select {
	case response = <-cViaCep:
		response = fmt.Sprintf("Recebido de ViaCep: %s\n", response)
	case response = <-cBrasilApi:
		response = fmt.Sprintf("Recebido de Brasil Api: %s\n", response)
	case <-time.After(time.Second * 1):
		response = ("Timeout na busca de CEP")
	}

	_, err := w.Write([]byte(response))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Erro geral na busca por CEP."))
		return
	}
	w.WriteHeader(http.StatusOK)
}

func SearchCepBrasilApi(cep string) (*BrasilApi, error) {
	// time.Sleep(1 * time.Second) -> para testar o timeout
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://brasilapi.com.br/api/cep/v1/"+cep, nil)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Brasil API retorna status 400 e 404 dependendo do formato da requisição.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil, errors.New("CEP não encontrado.")
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	var brasilApi BrasilApi
	err = json.Unmarshal(body, &brasilApi)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return &brasilApi, nil
}

func SearchCepViaCep(cep string) (*ViaCep, error) {
	// time.Sleep(1 * time.Second) -> para testar o timeout
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", "https://viacep.com.br/ws/"+cep+"/json/", nil)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		log.Println(err)
		return nil, err
	}

	var viaCep ViaCep
	err = json.Unmarshal(body, &viaCep)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return &viaCep, nil
}
