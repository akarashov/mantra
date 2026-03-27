package speech

// TranscriptionResponse представляет полный ответ транскрибации
type TranscriptionResponse struct {
	Results            []TranscriptionResult `json:"results"`
	EOU                bool                  `json:"eou"`
	EmotionsResult     EmotionsResult        `json:"emotions_result"`
	ProcessedAudioStart string                `json:"processed_audio_start"`
	ProcessedAudioEnd   string                `json:"processed_audio_end"`
	BackendInfo        BackendInfo           `json:"backend_info"`
	Channel            int                   `json:"channel"`
	SpeakerInfo        SpeakerInfo           `json:"speaker_info"`
	EOUReason          string                `json:"eou_reason"`
	Insight            string                `json:"insight"`
	PersonIdentity     PersonIdentity        `json:"person_identity"`
}

// TranscriptionResult представляет отдельный результат транскрибации
type TranscriptionResult struct {
	Text           string          `json:"text"`
	NormalizedText string          `json:"normalized_text"`
	Start          string          `json:"start"`
	End            string          `json:"end"`
	WordAlignments []WordAlignment `json:"word_alignments"`
}

// WordAlignment представляет выравнивание слов по времени
type WordAlignment struct {
	Word  string `json:"word"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// EmotionsResult содержит результаты анализа эмоций
type EmotionsResult struct {
	Positive float64 `json:"positive"`
	Neutral  float64 `json:"neutral"`
	Negative float64 `json:"negative"`
}

// BackendInfo содержит информацию о бэкенде
type BackendInfo struct {
	ModelName      string `json:"model_name"`
	ModelVersion   string `json:"model_version"`
	ServerVersion  string `json:"server_version"`
}

// SpeakerInfo содержит информацию о говорящем
type SpeakerInfo struct {
	SpeakerID             int     `json:"speaker_id"`
	MainSpeakerConfidence float64 `json:"main_speaker_confidence"`
}

// PersonIdentity содержит информацию о личности
type PersonIdentity struct {
	Age          string  `json:"age"`
	Gender       string  `json:"gender"`
	AgeScore     float64 `json:"age_score"`
	GenderScore  float64 `json:"gender_score"`
}

// Константы для возможных значений Age и Gender
const (
	AgeNone = "AGE_NONE"
	GenderNone = "GENDER_NONE"
)