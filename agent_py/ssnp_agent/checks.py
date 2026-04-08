from __future__ import annotations

import hashlib
import json
import os
import platform
import random
import shutil
import socket
import subprocess
import tempfile
import threading
import time
from dataclasses import dataclass
from multiprocessing import Process, Queue, Value
from pathlib import Path

from .policy import CPUProfile, DiskProfile, HardwareThresholds


@dataclass(slots=True)
class HardwareResult:
    cpu_check_passed: bool
    ram_check_passed: bool
    storage_size_check_passed: bool
    ssd_check_passed: bool
    visible_cpu_threads: int
    visible_memory_bytes: int
    visible_storage_bytes: int


@dataclass(slots=True)
class CPUResult:
    passed: bool
    normalized_score: float


@dataclass(slots=True)
class DiskResult:
    passed: bool
    measured_iops: float
    measured_latency_p95: float


def run_hardware(temp_dir: str, thresholds: HardwareThresholds) -> HardwareResult:
    threads = os.cpu_count() or 0
    memory = _visible_memory()
    storage = _visible_storage(temp_dir)
    ssd = _detect_ssd(temp_dir)
    gib = 1024 * 1024 * 1024
    return HardwareResult(
        cpu_check_passed=threads >= thresholds.cpu_cores_min,
        ram_check_passed=(memory // gib) >= thresholds.ram_gb_min,
        storage_size_check_passed=(storage // gib) >= thresholds.storage_gb_min,
        ssd_check_passed=(not thresholds.ssd_required) or ssd,
        visible_cpu_threads=threads,
        visible_memory_bytes=memory,
        visible_storage_bytes=storage,
    )


def run_cpu(profile: CPUProfile) -> CPUResult:
    workers = min(profile.worker_cap, os.cpu_count() or 0)
    if workers <= 0:
        return CPUResult(False, 0.0)

    phase = Value("i", 0)
    stop = Value("i", 0)
    queue: Queue[int] = Queue()
    processes = [
        Process(target=_cpu_worker, args=(worker_id, profile, phase, stop, queue))
        for worker_id in range(workers)
    ]
    for proc in processes:
        proc.start()

    _sleep(profile.warmup_seconds)
    phase.value = 1
    _sleep(profile.measured_seconds)
    phase.value = 2
    _sleep(profile.cooldown_seconds)
    stop.value = 1

    total = 0
    for _ in processes:
        total += queue.get()
    for proc in processes:
        proc.join()

    score = total / max(profile.measured_seconds, 1)
    return CPUResult(
        passed=score >= profile.acceptance_floor.minimum,
        normalized_score=float(score),
    )


def run_disk(temp_dir: str, profile: DiskProfile) -> DiskResult:
    Path(temp_dir).mkdir(parents=True, exist_ok=True)
    fd, path = tempfile.mkstemp(prefix="ssnp-disk-check-", dir=temp_dir)
    try:
        file_size = profile.block_size_bytes * profile.queue_depth * 256
        os.ftruncate(fd, file_size)
        ops = 0
        ops_lock = threading.Lock()
        stop = threading.Event()
        measure = threading.Event()

        def worker(seed: int) -> None:
            nonlocal ops
            rng = random.Random(seed)
            block = bytes(profile.block_size_bytes)
            block_slots = max(file_size // profile.block_size_bytes, 1)
            while not stop.is_set():
                offset = rng.randrange(block_slots) * profile.block_size_bytes
                if rng.random() < profile.read_ratio:
                    os.pread(fd, profile.block_size_bytes, offset)
                else:
                    os.pwrite(fd, block, offset)
                if measure.is_set():
                    with ops_lock:
                        ops += 1

        threads = [
            threading.Thread(target=worker, args=(int(time.time_ns()) + index,), daemon=True)
            for index in range(profile.concurrency)
        ]
        for thread in threads:
            thread.start()
        _sleep(profile.warmup_seconds)
        measure.set()
        _sleep(profile.measured_seconds)
        measure.clear()
        _sleep(profile.cooldown_seconds)
        stop.set()
        for thread in threads:
            thread.join()

        iops = ops / max(profile.measured_seconds, 1)
        return DiskResult(
            passed=iops >= profile.acceptance_floor.minimum,
            measured_iops=float(iops),
            measured_latency_p95=0.0,
        )
    except OSError:
        return DiskResult(False, 0.0, 0.0)
    finally:
        os.close(fd)
        try:
            os.remove(path)
        except FileNotFoundError:
            pass


def local_check_execution_failed(
    hardware: HardwareResult, cpu_result: CPUResult, disk_result: DiskResult
) -> bool:
    return (
        hardware.visible_cpu_threads == 0
        or hardware.visible_memory_bytes == 0
        or hardware.visible_storage_bytes == 0
        or (cpu_result.normalized_score == 0 and not cpu_result.passed)
        or (disk_result.measured_iops == 0 and not disk_result.passed)
    )


def _cpu_worker(
    worker_id: int, profile: CPUProfile, phase: Value, stop: Value, queue: Queue[int]
) -> None:
    seq = worker_id + 1
    total = 0
    while stop.value == 0:
        seq = (seq * 6364136223846793005 + 1) & 0xFFFFFFFFFFFFFFFF
        choice = (seq % 1000) / 1000.0
        if choice < profile.workload_mix.hashing:
            _do_hash(seq)
        elif choice < profile.workload_mix.hashing + profile.workload_mix.integer:
            _do_integer(seq)
        else:
            _do_matrix(seq)
        if phase.value == 1:
            total += 1
    queue.put(total)


def _do_hash(value: int) -> None:
    hashlib.sha256(value.to_bytes(8, "little") + bytes(56)).digest()


def _do_integer(value: int) -> None:
    current = value
    for index in range(64):
        current = ((current << 7) ^ (current >> 3)) + (index * index + 1)


def _do_matrix(value: int) -> None:
    left = [float(value % 7 + 1), 2, 3, 4, 5, 6, 7, 8, 9]
    right = [9, 8, 7, 6, 5, 4, 3, 2, 1]
    output = [0.0] * 9
    for row in range(3):
        for col in range(3):
            for idx in range(3):
                output[row * 3 + col] += left[row * 3 + idx] * right[idx * 3 + col]


def _visible_memory() -> int:
    if platform.system().lower() == "linux":
        try:
            for line in Path("/proc/meminfo").read_text(encoding="utf-8").splitlines():
                if line.startswith("MemTotal:"):
                    return int(line.split()[1]) * 1024
        except (FileNotFoundError, ValueError):
            return 0
    if platform.system().lower() == "darwin":
        try:
            out = subprocess.check_output(["sysctl", "-n", "hw.memsize"], text=True)
            return int(out.strip())
        except (subprocess.SubprocessError, ValueError):
            return 0
    return 0


def _visible_storage(path: str) -> int:
    usage = shutil.disk_usage(path)
    return usage.total


def _detect_ssd(path: str) -> bool:
    try:
        abs_path = str(Path(path).resolve())
        out = subprocess.check_output(
            ["lsblk", "-J", "-o", "ROTA,MOUNTPOINT"], text=True, stderr=subprocess.DEVNULL
        )
        payload = json.loads(out)
    except (subprocess.SubprocessError, FileNotFoundError, json.JSONDecodeError):
        return False
    for device in payload.get("blockdevices", []):
        match = _match_mount(device, abs_path)
        if match is not None:
            return not match
    return False


def _match_mount(device: dict, path: str) -> bool | None:
    mountpoint = device.get("mountpoint") or ""
    if mountpoint and path.startswith(mountpoint):
        return bool(device.get("rota"))
    for child in device.get("children", []):
        match = _match_mount(child, path)
        if match is not None:
            return match
    return None


def _sleep(seconds: int) -> None:
    if seconds > 0:
        time.sleep(seconds)
