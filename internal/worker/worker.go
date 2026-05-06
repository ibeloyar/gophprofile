package worker

//import "fmt"
//
//type AvatarUploadEvent struct {
//	AvatarID string `json:"avatar_id"`
//	UserID   string `json:"user_id"`
//	S3Key    string `json:"s3_key"`
//}
//
//type AvatarProcessEvent struct {
//	AvatarID   string         `json:"avatar_id"`
//	Operations []ProcessingOp `json:"operations"`
//}
//
//type AvatarDeleteEvent struct {
//	AvatarID string   `json:"avatar_id"`
//	S3Keys   []string `json:"s3_keys"`
//}
//
////// Пример отправки события после загрузки
////func (s *AvatarService) PublishUploadEvent(avatarID, userID, s3Key string) error {
////	event := AvatarUploadEvent{
////		AvatarID: avatarID,
////		UserID:   userID,
////		S3Key:    s3Key,
////	}
////
////	// Для RabbitMQ
////	return s.publisher.Publish(
////		"avatars.exchange",     // exchange
////		"avatar.uploaded",       // routing key
////		event,
////	)
////
////	// Для Kafka
////	return s.producer.Send(&sarama.ProducerMessage{
////		Topic: "avatar-events",
////		Key:   sarama.StringEncoder(avatarID),
////		Value: sarama.JSONEncoder(event),
////	})
////}
//
//type Worker struct {
//}
//
//func New() *Worker {
//	return &Worker{}
//}
//
//// Пример обработки события в worker
//func (w *Worker) HandleUploadEvent(event AvatarUploadEvent) error {
//	// Получаем метаданные из БД
//	avatar, err := w.repo.GetAvatar(event.AvatarID)
//	if err != nil {
//		return err
//	}
//
//	// Загружаем оригинал из S3
//	image, err := w.s3.Download(event.S3Key)
//	if err != nil {
//		return err
//	}
//
//	// Создаем миниатюры
//	thumbnails := []struct {
//		size string
//		data []byte
//	}{
//		{"100x100", w.resizer.Resize(image, 100, 100)},
//		{"300x300", w.resizer.Resize(image, 300, 300)},
//	}
//
//	// Сохраняем миниатюры в S3
//	for _, thumb := range thumbnails {
//		key := fmt.Sprintf("thumbnails/%s/%s.jpg", event.AvatarID, thumb.size)
//		if err := w.s3.Upload(key, thumb.data); err != nil {
//			return err
//		}
//	}
//
//	// Обновляем статус в БД
//	return w.repo.UpdateProcessingStatus(event.AvatarID, "completed")
//}
