#!/usr/bin/env python
"""Memory profiler for diagnosing container memory usage."""
import sys
import gc
import psutil
import tracemalloc

def main():
    tracemalloc.start()
    
    # Get current process memory
    process = psutil.Process()
    mem_info = process.memory_info()
    
    print(f"=== Process Memory ===")
    print(f"RSS (Resident Set Size): {mem_info.rss / 1024 / 1024:.2f} MB")
    print(f"VMS (Virtual Memory Size): {mem_info.vms / 1024 / 1024:.2f} MB")
    
    # Get system memory
    vm = psutil.virtual_memory()
    print(f"\n=== System Memory ===")
    print(f"Total: {vm.total / 1024 / 1024:.2f} MB")
    print(f"Used: {vm.used / 1024 / 1024:.2f} MB ({vm.percent}%)")
    print(f"Available: {vm.available / 1024 / 1024:.2f} MB")
    
    # Python object counts
    print(f"\n=== Python GC Stats ===")
    gc.collect()
    print(f"Collected objects: {gc.collect()}")
    print(f"GC counts: {gc.get_count()}")
    
    # Top memory allocations
    snapshot = tracemalloc.take_snapshot()
    top_stats = snapshot.statistics('lineno')
    
    print(f"\n=== Top 10 Memory Allocations ===")
    for i, stat in enumerate(top_stats[:10], 1):
        print(f"{i}. {stat.size / 1024 / 1024:.2f} MB - {stat}")

if __name__ == "__main__":
    main()
