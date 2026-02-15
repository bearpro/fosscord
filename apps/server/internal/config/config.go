package config

import "os"

type Config struct {
	Addr                      string
	ServerName                string
	PublicKeyFingerprintEmoji string
	DataDir                   string
	DatabasePath              string
	ServerPublicBaseURL       string
	AdminToken                string
	LiveKitURL                string
	LiveKitAPIKey             string
	LiveKitAPISecret          string
}

func Load() Config {
	return Config{
		Addr:                      getEnv("SERVER_ADDR", ":8080"),
		ServerName:                getEnv("SERVER_NAME", "Local Server"),
		PublicKeyFingerprintEmoji: getEnv("SERVER_PUBLIC_KEY_FINGERPRINT_EMOJI", ":lock::satellite:"),
		DataDir:                   getEnv("DATA_DIR", "data"),
		DatabasePath:              os.Getenv("DB_PATH"),
		ServerPublicBaseURL:       getEnv("SERVER_PUBLIC_BASE_URL", "http://localhost:8080"),
		AdminToken:                os.Getenv("ADMIN_TOKEN"),
		LiveKitURL:                getEnv("LIVEKIT_URL", "http://localhost:7880"),
		LiveKitAPIKey:             os.Getenv("LIVEKIT_API_KEY"),
		LiveKitAPISecret:          os.Getenv("LIVEKIT_API_SECRET"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
