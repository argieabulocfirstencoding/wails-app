package service_classes

type ScreenshotService struct {
}

func NewScreenshotService() *ScreenshotService {
	return &ScreenshotService{}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	return nil, nil
}

func (s *ScreenshotService) Start() {

}

func (s *ScreenshotService) Stop() {

}

func (s *ScreenshotService) AddMemo(memo, notes string) {

}
