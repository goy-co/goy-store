//! Native observability for Goy Store
//!
//! Every Goy Store operation emits metrics without additional code in consumer services.

use prometheus::{CounterVec, HistogramVec, GaugeVec, Registry, Opts, register_counter_vec_with_registry, register_histogram_vec_with_registry, register_gauge_vec_with_registry};
use anyhow::Result;
use std::sync::Arc;

pub struct StoreMetrics {
    pub operations_total: CounterVec,
    pub operation_duration_seconds: HistogramVec,
    pub pool_active_connections: GaugeVec,
    pub pool_idle_connections: GaugeVec,
    pub errors_total: CounterVec,
    pub retry_attempts_total: CounterVec,
}

impl StoreMetrics {
    pub fn new(registry: &Registry) -> Result<Self> {
        let operations_total = register_counter_vec_with_registry!(
            Opts::new("goy_store_operations_total", "Total number of store operations"),
            &["contract", "operation", "backend", "status"],
            registry
        )?;

        let operation_duration_seconds = register_histogram_vec_with_registry!(
            Opts::new("goy_store_operation_duration_seconds", "Duration of store operations in seconds"),
            &["contract", "operation", "backend"],
            registry
        )?;

        let pool_active_connections = register_gauge_vec_with_registry!(
            Opts::new("goy_store_pool_active_connections", "Number of active connections in the pool"),
            &["backend"],
            registry
        )?;

        let pool_idle_connections = register_gauge_vec_with_registry!(
            Opts::new("goy_store_pool_idle_connections", "Number of idle connections in the pool"),
            &["backend"],
            registry
        )?;

        let errors_total = register_counter_vec_with_registry!(
            Opts::new("goy_store_errors_total", "Total number of store errors"),
            &["backend", "error_type"],
            registry
        )?;

        let retry_attempts_total = register_counter_vec_with_registry!(
            Opts::new("goy_store_retry_attempts_total", "Total number of retry attempts"),
            &["backend", "operation"],
            registry
        )?;

        Ok(Self {
            operations_total,
            operation_duration_seconds,
            pool_active_connections,
            pool_idle_connections,
            errors_total,
            retry_attempts_total,
        })
    }
}

impl Default for StoreMetrics {
    fn default() -> Self {
        let registry = Registry::new();
        Self::new(&registry).expect("Failed to create default metrics")
    }
}