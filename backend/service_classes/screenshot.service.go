package service_classes

import "context"

type ScreenshotService struct {
	FolderName   string
	TaskID       *string
	TaskRecordID *uint
	ctx          context.Context
	Running      bool
	Memo         string
	Notes        string
}

func NewScreenshotService() *ScreenshotService {
	return &ScreenshotService{}
}

func (s *ScreenshotService) Screenshot() ([]string, error) {
	return nil, nil
}

func (s *ScreenshotService) Start(ctx context.Context) {
	s.ctx = ctx
}

func (s *ScreenshotService) Stop() {

}

func (s *ScreenshotService) AddMemo(memo, notes string) {

}
