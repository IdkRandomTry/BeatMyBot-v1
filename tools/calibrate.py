#!/usr/bin/env python3
"""
Calibration micro-benchmark
Prints a small JSON with measured seconds, e.g. {"seconds":0.1234}
Run inside the same Docker image used for bots to get a baseline.
"""
import time
import json

def bench(iters=200_000):
    s = time.perf_counter()
    x = 0
    for i in range(iters):
        x += (i % 7) * (i % 11)
    e = time.perf_counter()
    return e - s

if __name__ == '__main__':
    t = bench()
    print(json.dumps({"seconds": t}))
