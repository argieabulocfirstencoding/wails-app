package serviceclasses

import "net/http"

type FesClientService struct {
	baseUrl string
	client  *http.Client
}

func NewFesClientService() *FesClientService {
	return &FesClientService{
		baseUrl: "https://firstencoding.app/api/1.1/wf/",
		client:  &http.Client{},
	}
}

func (f *FesClientService) Request(reqType, pathname string) {

}
