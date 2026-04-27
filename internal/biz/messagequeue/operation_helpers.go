package messagequeue

import (
	"encoding/json"
	"fmt"
	"strings"
)

func normalizeResourceOperationRequest(req *ResourceOperationRequest) {
	if req == nil {
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	req.ResourceType = strings.TrimSpace(req.ResourceType)
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.ResourceName = strings.TrimSpace(req.ResourceName)
	req.Reason = strings.TrimSpace(req.Reason)
	req.ConfirmText = strings.TrimSpace(req.ConfirmText)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.Params == nil {
		req.Params = map[string]any{}
	}
}

func newOperationValidation(instance *MQInstance, req *ResourceOperationRequest, riskLevel string) *ResourceOperationValidationVO {
	validation := &ResourceOperationValidationVO{
		InstanceID:         instance.ID,
		MQType:             instance.MQType,
		Action:             req.Action,
		ActionText:         actionText(req.Action),
		RiskLevel:          riskLevel,
		RequiredPermission: PermissionResourceManage,
		ResourceType:       req.ResourceType,
		Namespace:          req.Namespace,
		ResourceName:       req.ResourceName,
		Supported:          true,
		NormalizedParams:   cloneParams(req.Params),
		RequiresConfirm:    true,
	}
	if riskLevel == RiskLevelHigh || riskLevel == RiskLevelCritical {
		validation.RequiresHighRiskAck = true
	}
	return validation
}

func IsHighRiskLevel(riskLevel string) bool {
	switch strings.ToLower(strings.TrimSpace(riskLevel)) {
	case RiskLevelHigh, RiskLevelCritical:
		return true
	default:
		return false
	}
}

func unsupportedOperationValidation(instance *MQInstance, req *ResourceOperationRequest, message string) *ResourceOperationValidationVO {
	validation := newOperationValidation(instance, req, RiskLevelLow)
	validation.Supported = false
	validation.Message = message
	return validation
}

func ensureOperationRequired(value, label string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s不能为空", label)
	}
	return nil
}

func cloneParams(params map[string]any) map[string]any {
	if len(params) == 0 {
		return map[string]any{}
	}
	result := make(map[string]any, len(params))
	for key, value := range params {
		result[key] = value
	}
	return result
}

func operationStringParam(params map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			return strings.TrimSpace(stringValue(value))
		}
	}
	return ""
}

func operationParamExists(params map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := params[key]; ok {
			return true
		}
	}
	return false
}

func operationBoolParam(params map[string]any, fallback bool, keys ...string) bool {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			return boolValue(value)
		}
	}
	return fallback
}

func operationIntParam(params map[string]any, fallback int, keys ...string) int {
	for _, key := range keys {
		if value, ok := params[key]; ok {
			return intValue(value)
		}
	}
	return fallback
}

func operationStringMapParam(params map[string]any, keys ...string) map[string]string {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch item := value.(type) {
		case map[string]string:
			return item
		case map[string]any:
			result := make(map[string]string, len(item))
			for k, v := range item {
				result[strings.TrimSpace(k)] = stringValue(v)
			}
			return result
		case string:
			var result map[string]string
			if err := json.Unmarshal([]byte(item), &result); err == nil {
				return result
			}
		}
	}
	return map[string]string{}
}

func operationAnyMapParam(params map[string]any, keys ...string) map[string]any {
	for _, key := range keys {
		value, ok := params[key]
		if !ok || value == nil {
			continue
		}
		switch item := value.(type) {
		case map[string]any:
			return item
		case map[string]string:
			result := make(map[string]any, len(item))
			for k, v := range item {
				result[k] = v
			}
			return result
		case string:
			var result map[string]any
			if err := json.Unmarshal([]byte(item), &result); err == nil {
				return result
			}
		}
	}
	return map[string]any{}
}

func setNormalizedParams(req *ResourceOperationRequest, validation *ResourceOperationValidationVO, params map[string]any) {
	if req != nil {
		req.Params = params
	}
	if validation != nil {
		validation.NormalizedParams = cloneParams(params)
	}
}
