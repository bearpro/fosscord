package config

import "os"

type Config struct {
	Addr                      string
	ServerName                string
	PublicKeyFingerprintEmoji string
	LiveKitURL                string
	LiveKitAPIKey             string
	LiveKitAPISecret          string
}

func Load() Config {
	return Config{
		Addr:                      getEnv("SERVER_ADDR", ":8080"),
		ServerName:                getEnv("SERVER_NAME", "Local Server"),
		PublicKeyFingerprintEmoji: getEnv("SERVER_PUBLIC_KEY_FINGERPRINT_EMOJI", ":lock::satellite:"),
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
