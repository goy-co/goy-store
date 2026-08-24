//! Sorted Set Store Contract
//!
//! Sets ordered by score (typically timestamp). Used for heartbeats, stale node detection,
//! priority queues, contribution leaderboards.

use anyhow::Result;
use async_trait::async_trait;
use std::collections::BTreeMap;
use std::sync::Arc;
use tokio::sync::RwLock;

#[async_trait]
pub trait SortedSetStore: Send + Sync {
    async fn add(&self, set: &str, member: &str, score: f64) -> Result<()>;
    async fn remove(&self, set: &str, member: &str) -> Result<()>;
    async fn range_by_score(
        &self,
        set: &str,
        min: f64,
        max: f64,
        limit: Option<usize>,
    ) -> Result<Vec<(String, f64)>>;
    async fn count(&self, set: &str) -> Result<u64>;
    async fn remove_range(&self, set: &str, min: f64, max: f64) -> Result<u64>;
}

#[derive(Default)]
struct InnerSet {
    // Maps score to a list of members with that score
    scores: BTreeMap<f64, Vec<String>>,
    // Maps member to their current score for O(1) lookups and removals
    member_scores: std::collections::HashMap<String, f64>,
}

/// In-memory implementation of SortedSetStore for testing and local development.
#[derive(Default)]
pub struct MemorySortedSetStore {
    sets: Arc<RwLock<std::collections::HashMap<String, InnerSet>>>,
}

#[async_trait]
impl SortedSetStore for MemorySortedSetStore {
    async fn add(&self, set: &str, member: &str, score: f64) -> Result<()> {
        let mut sets = self.sets.write().await;
        let inner = sets.entry(set.to_string()).or_default();
        
        // Remove old score if member already exists
        if let Some(old_score) = inner.member_scores.get(member) {
            if let Some(members) = inner.scores.get_mut(old_score) {
                members.retain(|m| m != member);
                if members.is_empty() {
                    inner.scores.remove(old_score);
                }
            }
        }
        
        inner.member_scores.insert(member.to_string(), score);
        inner.scores.entry(score).or_default().push(member.to_string());
        
        Ok(())
    }

    async fn remove(&self, set: &str, member: &str) -> Result<()> {
        let mut sets = self.sets.write().await;
        if let Some(inner) = sets.get_mut(set) {
            if let Some(score) = inner.member_scores.remove(member) {
                if let Some(members) = inner.scores.get_mut(&score) {
                    members.retain(|m| m != member);
                    if members.is_empty() {
                        inner.scores.remove(&score);
                    }
                }
            }
        }
        Ok(())
    }

    async fn range_by_score(
        &self,
        set: &str,
        min: f64,
        max: f64,
        limit: Option<usize>,
    ) -> Result<Vec<(String, f64)>> {
        let sets = self.sets.read().await;
        let mut result = Vec::new();
        
        if let Some(inner) = sets.get(set) {
            for (&score, members) in inner.scores.range(min..=max) {
                for member in members {
                    result.push((member.clone(), score));
                    if let Some(limit) = limit {
                        if result.len() >= limit {
                            return Ok(result);
                        }
                    }
                }
            }
        }
        
        Ok(result)
    }

    async fn count(&self, set: &str) -> Result<u64> {
        let sets = self.sets.read().await;
        if let Some(inner) = sets.get(set) {
            Ok(inner.member_scores.len() as u64)
        } else {
            Ok(0)
        }
    }

    async fn remove_range(&self, set: &str, min: f64, max: f64) -> Result<u64> {
        let mut sets = self.sets.write().await;
        let mut removed_count = 0;
        
        if let Some(inner) = sets.get_mut(set) {
            let scores_to_remove: Vec<f64> = inner.scores.keys().copied().filter(|&s| s >= min && s <= max).collect();
            
            for score in scores_to_remove {
                if let Some(members) = inner.scores.remove(&score) {
                    removed_count += members.len() as u64;
                    for member in members {
                        inner.member_scores.remove(&member);
                    }
                }
            }
        }
        
        Ok((removed_count))
    }
}