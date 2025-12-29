'use client';

import { useState, useEffect, useCallback } from 'react';

interface ConnectionWarningProps {
  onStatusChange?: (connected: boolean) => void;
}

export function ConnectionWarning({ onStatusChange }: ConnectionWarningProps) {
  const [isConnected, setIsConnected] = useState<boolean | null>(null);
  const [retrying, setRetrying] = useState(false);

  const checkConnection = useCallback(async () => {
    try {
      const controller = new AbortController();
      const timeoutId = setTimeout(() => controller.abort(), 5000);
      
      const response = await fetch('/api/healthz', {
        signal: controller.signal,
      });
      clearTimeout(timeoutId);
      
      const connected = response.ok;
      setIsConnected(connected);
      onStatusChange?.(connected);
      return connected;
    } catch {
      setIsConnected(false);
      onStatusChange?.(false);
      return false;
    }
  }, [onStatusChange]);

  const handleRetry = async () => {
    setRetrying(true);
    await checkConnection();
    setRetrying(false);
  };

  useEffect(() => {
    checkConnection();
    
    // Re-check every 10 seconds if disconnected
    const interval = setInterval(() => {
      if (!isConnected) {
        checkConnection();
      }
    }, 10000);
    
    return () => clearInterval(interval);
  }, [isConnected, checkConnection]);

  // Don't show anything while checking or if connected
  if (isConnected === null || isConnected) {
    return null;
  }

  return (
    <div className="connection-warning">
      <div className="connection-warning-icon">🔌</div>
      <div className="connection-warning-content">
        <h3>Cannot connect to Rice Search API</h3>
        <p>
          Make sure the API server is running on port 8088.
        </p>
        <div className="connection-warning-commands">
          <code>cd api && bun run start:local</code>
        </div>
        <p className="connection-warning-note">
          Also ensure backend services are running:
        </p>
        <div className="connection-warning-commands">
          <code>docker-compose up -d milvus embeddings etcd minio</code>
        </div>
      </div>
      <button 
        className="connection-warning-retry"
        onClick={handleRetry}
        disabled={retrying}
      >
        {retrying ? 'Checking...' : 'Retry'}
      </button>
    </div>
  );
}
