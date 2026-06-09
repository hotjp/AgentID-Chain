// Package telemetry: Span 属性标准化 (P19.2)。
//
// 基于 OTel 语义约定（Semantic Conventions）定义统一的 span attribute 命名。
// 避免各业务模块自己发明 key，方便跨服务关联。
//
// 参考：
//   - https://opentelemetry.io/docs/specs/semconv/
//   - https://opentelemetry.io/docs/specs/semconv/http/http-spans/
//   - https://opentelemetry.io/docs/specs/semconv/database/
package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
)

// =============================================================================
// 服务级属性
// =============================================================================

const (
	AttrServiceName      = "service.name"
	AttrServiceVersion   = "service.version"
	AttrServiceNamespace = "service.namespace"
	AttrServiceInstance  = "service.instance.id"
)

// ServiceAttrs 返回服务标识属性集。
func ServiceAttrs(name, version, namespace, instanceID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrServiceName, name),
		attribute.String(AttrServiceVersion, version),
		attribute.String(AttrServiceNamespace, namespace),
		attribute.String(AttrServiceInstance, instanceID),
	}
}

// =============================================================================
// HTTP 属性（遵循 OTel HTTP 语义约定）
// =============================================================================

const (
	AttrHTTPRequestMethod      = "http.request.method"
	AttrHTTPResponseStatusCode = "http.response.status_code"
	AttrHTTPRoute               = "http.route"
	AttrHTTPTarget              = "http.target"
	AttrHTTPScheme              = "http.scheme"
	AttrHTTPUserAgent           = "user_agent.original"
	AttrHTTPClientIP            = "client.address"
	AttrHTTPServerAddr          = "server.address"
)

// HTTPRequestAttrs 返回 HTTP 请求属性集。
func HTTPRequestAttrs(method, route, target, scheme string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrHTTPRequestMethod, method),
		attribute.String(AttrHTTPRoute, route),
		attribute.String(AttrHTTPTarget, target),
		attribute.String(AttrHTTPScheme, scheme),
	}
}

// HTTPResponseAttrs 返回 HTTP 响应属性集。
func HTTPResponseAttrs(statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int(AttrHTTPResponseStatusCode, statusCode),
	}
}

// =============================================================================
// 数据库属性（遵循 OTel Database 语义约定）
// =============================================================================

const (
	AttrDBSystem    = "db.system"
	AttrDBName      = "db.namespace"
	AttrDBStatement = "db.statement"
	AttrDBOperation = "db.operation"
	AttrDBResponseStatus = "db.response.status_code"
	AttrDBRowsAffected   = "db.rows_affected"
)

// DBAttrs 返回数据库操作属性集。
func DBAttrs(system, name, operation, statement string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrDBSystem, system),
		attribute.String(AttrDBName, name),
		attribute.String(AttrDBOperation, operation),
		attribute.String(AttrDBStatement, statement),
	}
}

// =============================================================================
// AgentID-Chain 业务属性
// =============================================================================

const (
	// AgentID-Chain 业务域
	AttrAgentID         = "agentid.uuid"
	AttrAgentLevel      = "agentid.level"
	AttrAgentOwner      = "agentid.owner"
	AttrAgentState      = "agentid.state"

	// AAP 鉴权
	AttrAAPVersion      = "aap.version"
	AttrAAPChallengeID  = "aap.challenge_id"
	AttrAAPDomain       = "aap.domain"
	AttrAAPResult       = "aap.result"         // success | failure
	AttrAAPFailureReason = "aap.failure_reason"

	// A2A
	AttrA2ASessionID   = "a2a.session_id"
	AttrA2ACounterparty = "a2a.counterparty"
	AttrA2AOperation   = "a2a.operation"

	// 链上
	AttrChainType      = "chain.type"
	AttrChainTxHash    = "chain.tx_hash"
	AttrChainBlockNum  = "chain.block_number"
	AttrChainMethod    = "chain.method"
	AttrChainNetwork   = "chain.network"

	// 缓存
	AttrCacheSystem    = "cache.system"
	AttrCacheHit       = "cache.hit"
	AttrCacheKey       = "cache.key"        // 注意 PII 风险
	AttrCacheOperation = "cache.operation"

	// 限流
	AttrRateLimitKey   = "ratelimit.key"
	AttrRateLimitValue = "ratelimit.value"
	AttrRateLimitLimit = "ratelimit.limit"
	AttrRateLimitRemaining = "ratelimit.remaining"

	// 业务
	AttrBusinessAction = "business.action"
	AttrBusinessResult = "business.result"
)

// AgentAttrs 返回 Agent 相关属性。
func AgentAttrs(uuid, owner, level, state string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrAgentID, uuid),
		attribute.String(AttrAgentOwner, owner),
		attribute.String(AttrAgentLevel, level),
		attribute.String(AttrAgentState, state),
	}
}

// AAPAttrs 返回 AAP 操作属性。
func AAPAttrs(version, challengeID, domain, result, reason string) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrAAPVersion, version),
		attribute.String(AttrAAPChallengeID, challengeID),
		attribute.String(AttrAAPDomain, domain),
		attribute.String(AttrAAPResult, result),
	}
	if reason != "" {
		attrs = append(attrs, attribute.String(AttrAAPFailureReason, reason))
	}
	return attrs
}

// ChainAttrs 返回链上操作属性。
func ChainAttrs(chainType, method, network, txHash string, blockNum uint64) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrChainType, chainType),
		attribute.String(AttrChainMethod, method),
		attribute.String(AttrChainNetwork, network),
		attribute.String(AttrChainTxHash, txHash),
		attribute.Int64(AttrChainBlockNum, int64(blockNum)),
	}
}

// CacheAttrs 返回缓存操作属性。
func CacheAttrs(system, operation string, hit bool) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrCacheSystem, system),
		attribute.String(AttrCacheOperation, operation),
		attribute.Bool(AttrCacheHit, hit),
	}
}
