//! Health check contract and status models.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum HealthState {
    Healthy,
    Degraded,
    Unhealthy,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HealthStatus {
    pub contract: String,
    pub backend: String,
    pub state: HealthState,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub message: Option<String>,
    pub latency_ms: u64,
}

impl HealthStatus {
    pub fn healthy(contract: &str, backend: &str, latency_ms: u64) -> Self {
        Self {
            contract: contract.to_string(),
            backend: backend.to_string(),
            state: HealthState::Healthy,
            message: None,
            latency_ms,
        }
    }

    pub fn unhealthy(contract: &str, backend: &str, message: &str, latency_ms: u64) -> Self {
        Self {
            contract: contract.to_string(),
            backend: backend.to_string(),
            state: HealthState::Unhealthy,
            message: Some(message.to_string()),
            latency_ms,
        }
    }

    pub fn degraded(contract: &str, backend: &str, message: &str, latency_ms: u64) -> Self {
        Self {
            contract: contract.to_string(),
            backend: backend.to_string(),
            state: HealthState::Degraded,
            message: Some(message.to_string()),
            latency_ms,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ConsolidatedHealth {
    pub state: HealthState,
    pub contracts: HashMap<String, HealthStatus>,
}

impl ConsolidatedHealth {
    pub fn from_statuses(statuses: Vec<HealthStatus>) -> Self {
        let mut overall_state = HealthState::Healthy;
        let mut map = HashMap::new();

        for status in statuses {
            match status.state {
                HealthState::Unhealthy => overall_state = HealthState::Unhealthy,
                HealthState::Degraded if overall_state != HealthState::Unhealthy => {
                    overall_state = HealthState::Degraded;
                }
                _ => {}
            }
            map.insert(status.contract.clone(), status);
        }

        Self {
            state: overall_state,
            contracts: map,
        }
    }
}
