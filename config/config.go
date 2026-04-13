package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	OpenAIAPIKey         string
	MineaEmail           string
	MineaPassword        string
	SupAPIKey            string
	MetaAccessToken      string
	MetaAdAccountID      string
	TikTokAccessToken    string
	ShopifyTechDomain    string
	ShopifyPetsDomain    string
	NotifyEmail          string
	NotifySMTPHost       string
	NotifySMTPPort       int
	NotifySMTPUser       string
	NotifySMTPPassword   string
	NotifySlackWebhook   string
	NotifyDiscordWebhook string
	NotifyTelegramToken  string
	NotifyTelegramChatID string
	NotifyMinSeverity    string
	DatabasePath         string
	AgentIntervalHours   int
	AutoApprove          bool
	Port                 string
	DevMode              bool
}

func Load() (*Config, error) {
	cfg := &Config{
		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		MineaEmail:           os.Getenv("MINEA_EMAIL"),
		MineaPassword:        os.Getenv("MINEA_PASSWORD"),
		SupAPIKey:            os.Getenv("SUP_API_KEY"),
		MetaAccessToken:      os.Getenv("META_ACCESS_TOKEN"),
		MetaAdAccountID:      os.Getenv("META_AD_ACCOUNT_ID"),
		TikTokAccessToken:    os.Getenv("TIKTOK_ACCESS_TOKEN"),
		ShopifyTechDomain:    os.Getenv("SHOPIFY_TECH_DOMAIN"),
		ShopifyPetsDomain:    os.Getenv("SHOPIFY_PETS_DOMAIN"),
		NotifyEmail:          os.Getenv("NOTIFY_EMAIL"),
		NotifySMTPHost:       getEnvOrDefault("NOTIFY_SMTP_HOST", "smtp.gmail.com"),
		NotifySMTPPort:       getEnvAsInt("NOTIFY_SMTP_PORT", 587),
		NotifySMTPUser:       os.Getenv("NOTIFY_SMTP_USER"),
		NotifySMTPPassword:   os.Getenv("NOTIFY_SMTP_PASSWORD"),
		NotifySlackWebhook:   os.Getenv("NOTIFY_SLACK_WEBHOOK"),
		NotifyDiscordWebhook: os.Getenv("NOTIFY_DISCORD_WEBHOOK"),
		NotifyTelegramToken:  os.Getenv("NOTIFY_TELEGRAM_BOT_TOKEN"),
		NotifyTelegramChatID: os.Getenv("NOTIFY_TELEGRAM_CHAT_ID"),
		NotifyMinSeverity:    getEnvOrDefault("NOTIFY_MIN_SEVERITY", "critical"),
		DatabasePath:         getEnvOrDefault("DB_PATH", "./data/agent.db"),
		Port:                 getEnvOrDefault("PORT", "8080"),
		DevMode:              getEnvAsBool("DEV_MODE", true),
		AutoApprove:          getEnvAsBool("AUTO_APPROVE", false),
		AgentIntervalHours:   getEnvAsInt("AGENT_INTERVAL_HOURS", 6),
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.DevMode {
		return nil
	}

	var missing []string
	if strings.TrimSpace(c.OpenAIAPIKey) == "" {
		missing = append(missing, "OPENAI_API_KEY")
	}
	if strings.TrimSpace(c.MineaEmail) == "" {
		missing = append(missing, "MINEA_EMAIL")
	}
	if strings.TrimSpace(c.MineaPassword) == "" {
		missing = append(missing, "MINEA_PASSWORD")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getEnvAsInt(key string, defaultValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
