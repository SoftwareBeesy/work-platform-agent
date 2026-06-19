package ops

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SoftwareBeesy/work-platform-agent/internal/contract"
	"github.com/SoftwareBeesy/work-platform-agent/internal/occ"
)

const memailAppID = "mework360_memail"

// OccExecutor runs allowlisted occ-exec subcommands.
type OccExecutor interface {
	Exec(ctx context.Context, tenantSlug, subcmd string, args []string) (map[string]any, error)
}

// HandleMemailConfigure applies ISSUE-024 meMail settings idempotently via OccAdapter.
func HandleMemailConfigure(
	ctx context.Context,
	cmd contract.Command,
	farmID string,
	occExec OccExecutor,
	emit EventEmitter,
) error {
	cfg, err := parseMemailPayload(cmd.Payload)
	if err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "validate", err.Error())
	}

	if err := emit(ctx, progressEvent(cmd, farmID, "running", "accepted", "memail configure accepted", 5)); err != nil {
		return err
	}

	steps := []string{"accepted"}

	if err := setAppConfig(ctx, occExec, cfg, "externalLocation", cfg.externalLocation); err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "external_location", err.Error())
	}
	steps = append(steps, "external_location")

	forceValue := "yes"
	if !cfg.forceSSO {
		forceValue = "no"
	}
	if err := setAppConfig(ctx, occExec, cfg, "forceSSO", forceValue); err != nil {
		return emitFailed(ctx, emit, cmd, farmID, "force_sso", err.Error())
	}
	steps = append(steps, "force_sso")

	if cfg.emailAddressChoice != "" {
		if err := setAppConfig(ctx, occExec, cfg, "emailAddressChoice", cfg.emailAddressChoice); err != nil {
			return emitFailed(ctx, emit, cmd, farmID, "email_address_choice", err.Error())
		}
		steps = append(steps, "email_address_choice")
	}

	if cfg.disableCoreMail {
		if _, err := occExec.Exec(ctx, cfg.tenantSlug, "app:disable", []string{"mail"}); err != nil {
			return emitFailed(ctx, emit, cmd, farmID, "disable_mail", err.Error())
		}
		steps = append(steps, "disable_mail")
	}

	return emit(ctx, contract.Event{
		SchemaVersion: 1,
		OperationID:   cmd.OperationID,
		FarmID:        farmID,
		State:         "succeeded",
		Step:          "memail_configured",
		Message:       "memail configuration applied",
		Percent:       100,
		Timestamp:     nowRFC3339(),
		EventType:     "progress",
		Data: map[string]any{
			"steps_completed": steps,
		},
	})
}

type memailConfig struct {
	tenantSlug         string
	externalLocation   string
	forceSSO           bool
	emailAddressChoice string
	disableCoreMail    bool
}

func parseMemailPayload(payload map[string]interface{}) (memailConfig, error) {
	if payload == nil {
		return memailConfig{}, fmt.Errorf("payload is required")
	}

	slug := stringField(payload, "tenant_slug")
	if slug == "" {
		return memailConfig{}, fmt.Errorf("tenant_slug is required")
	}

	external := stringField(payload, "external_location")
	if external == "" {
		external = strings.TrimSpace(os.Getenv("MEMAIL_EXTERNAL_LOCATION"))
	}
	if external == "" {
		return memailConfig{}, fmt.Errorf("external_location is required")
	}

	return memailConfig{
		tenantSlug:         slug,
		externalLocation:   external,
		forceSSO:           boolField(payload, "force_sso"),
		emailAddressChoice: stringField(payload, "email_address_choice"),
		disableCoreMail:    boolField(payload, "disable_core_mail_app"),
	}, nil
}

func setAppConfig(
	ctx context.Context,
	occExec OccExecutor,
	cfg memailConfig,
	setting string,
	value string,
) error {
	current, err := readAppConfig(ctx, occExec, cfg.tenantSlug, setting)
	if err != nil {
		return err
	}
	if strings.TrimSpace(current) == strings.TrimSpace(value) {
		return nil
	}

	_, err = occExec.Exec(ctx, cfg.tenantSlug, "config:app:set", []string{
		memailAppID,
		setting,
		"--value=" + value,
	})

	return err
}

func readAppConfig(
	ctx context.Context,
	occExec OccExecutor,
	tenantSlug string,
	setting string,
) (string, error) {
	parsed, err := occExec.Exec(ctx, tenantSlug, "config:app:get", []string{memailAppID, setting})
	if err != nil {
		return "", err
	}

	return configValueFromParsed(parsed), nil
}

func configValueFromParsed(parsed map[string]any) string {
	if parsed == nil {
		return ""
	}
	for _, key := range []string{"value", "stdout", "data"} {
		if raw, ok := parsed[key]; ok {
			if s, ok := raw.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// Ensure occ.Adapter satisfies OccExecutor.
var _ OccExecutor = (*occ.Adapter)(nil)
