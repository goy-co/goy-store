use super::common::*;
use std::time::Duration;
use tokio_stream::StreamExt;

#[tokio::test]
async fn test_redis_pubsub_messaging() {
    let store = create_test_store().await.expect("failed to create store");

    let channel = "events:cluster";
    let mut stream1 = store.pubsub.subscribe(&[channel]).await.unwrap();
    let mut stream2 = store.pubsub.subscribe(&[channel]).await.unwrap();

    // Small delay to ensure subscription is registered in redis
    tokio::time::sleep(Duration::from_millis(50)).await;

    // Publish message
    store.pubsub.publish(channel, b"test-event-payload").await.unwrap();

    // Receive on stream1
    let msg1 = tokio::time::timeout(Duration::from_secs(2), stream1.next()).await;
    assert!(msg1.is_ok(), "stream1 timed out waiting for message");
    let msg1 = msg1.unwrap().unwrap().unwrap();
    assert_eq!(msg1.channel, channel);
    assert_eq!(msg1.payload, b"test-event-payload");

    // Receive on stream2
    let msg2 = tokio::time::timeout(Duration::from_secs(2), stream2.next()).await;
    assert!(msg2.is_ok(), "stream2 timed out waiting for message");
    let msg2 = msg2.unwrap().unwrap().unwrap();
    assert_eq!(msg2.channel, channel);
    assert_eq!(msg2.payload, b"test-event-payload");

    // Unsubscribe
    store.pubsub.unsubscribe(&[channel]).await.unwrap();
}
