# Device Info Display Implementation

## Summary

Added device information display to the admin UI showing ML runtime device status (GPU/CPU usage and fallback state).

## Changes Made

### 1. Protocol Buffers (`api/proto/ricesearch.proto`)
- Added `DeviceInfo` message with fields:
  - `device` - Requested device (cpu, cuda, tensorrt)
  - `actual_device` - Device actually being used  
  - `device_fallback` - True if actual differs from requested
  - `runtime_available` - True if ONNX Runtime is available
- Added `device_info` field to `ComponentHealth` message

### 2. gRPC Adapter (`internal/server/grpc_adapter.go`)
- Updated ML health check (lines 243-258) to populate `DeviceInfo` from `ml.Health()`
- Passes device status through ComponentHealth protobuf message

### 3. Web Data Structures (`internal/web/stats.templ`)
- Extended `HealthStatus` struct with device fields:
  - `Device` - Requested device
  - `ActualDevice` - Actual device being used
  - `DeviceFallback` - Fallback indicator
  - `RuntimeAvail` - ONNX Runtime availability

### 4. Web Handlers (`internal/web/handlers.go`)
- Updated `handleDashboard` (lines 198-221) to extract device info
- Updated `getStatsData` (lines 2165-2191) to extract device info
- Added TODO comments for uncommenting after protobuf regeneration

### 5. Dashboard Template (`internal/web/dashboard.templ`)
- Enhanced `HealthCard` component to display device info when available
- Shows requested vs actual device
- Visual indicators:
  - Green checkmark: Device matches request
  - Yellow warning: Device fallback occurred
  - Yellow alert banner: Explains fallback
  - Red error banner: ONNX Runtime unavailable

### 6. Stats Page Template (`internal/web/stats.templ`)  
- Enhanced `HealthRow` component to display device info
- Shows requested and actual device in a collapsible format
- Color-coded status indicators matching dashboard

## Visual Design

### Dashboard ML Card
```
┌─────────────────────────┐
│ ● ML Models             │
│ degraded     15ms       │
│ GPU requested but...    │
│ ─────────────────────   │
│ Requested: cuda         │
│ Actual: ⚠ cpu          │
│ ⚠ Device fallback...    │
└─────────────────────────┘
```

### Stats Page ML Row
```
● ml                degraded     15ms
  ├─ Requested: cuda
  └─ Actual: ⚠ cpu
     ⚠ Device fallback: Requested cuda but using cpu
```

## Status Indicators

| Condition | Icon | Color | Message |
|-----------|------|-------|---------|
| Match | ✓ | Green | Device matches request |
| Fallback | ⚠ | Yellow | Fallback occurred |
| Runtime Missing | ✗ | Red | ONNX Runtime unavailable |

## Next Steps

**User action required:**
1. Regenerate protobuf files with `make proto` (requires protoc)
2. Uncomment the device info extraction code in handlers.go (search for "TODO")
3. Rebuild the server

## Testing

1. Start server with `RICE_ML_DEVICE=cuda` (when no GPU available)
2. View dashboard at `/` or stats at `/stats`
3. ML Models card should show:
   - Degraded status (yellow)
   - Requested: cuda
   - Actual: cpu (with warning icon)
   - Yellow banner: "Device fallback occurred"

## Files Modified

- `api/proto/ricesearch.proto`
- `internal/server/grpc_adapter.go`
- `internal/web/stats.templ`
- `internal/web/handlers.go`
- `internal/web/dashboard.templ`

## Dependencies

- Requires protobuf regeneration (user must run `make proto`)
- Requires ML service Health() returning device info (already implemented)
