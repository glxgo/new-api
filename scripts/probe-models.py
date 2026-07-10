#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
模型状态探测脚本 (probe-models.py)
周期性请求「每个模型 × 每个分组」，填充 perf_metrics + 检测可用性。

部署：
  /opt/newapi/probe-models.py + probe-keys.json (权限600)
  crontab: 7 * * * * /usr/bin/python3 /opt/newapi/probe-models.py   # 每小时一次

策略：
  - 按次付费模型(quota_type==1, model_price) 不探测：单次调用成本固定且高，
    探测无意义还会产生真实扣费，直接跳过。
  - 黑名单(图片/语音/embedding 等非 chat 模型) 不探测。
  - 每个请求间隔 SLEEP_BETWEEN，避免瞬时并发压垮 new-api/mysql。
  - 单分组连续失败 N 次则熔断跳过该分组剩余模型（高峰期不浪费）。
"""
import json
import os
import sys
import time
import urllib.error
import urllib.request

API_BASE = os.environ.get("PROBE_API_BASE", "http://127.0.0.1:3000")
KEYS_FILE = "/opt/newapi/probe-keys.json"
LOG = "/var/log/newapi-probe.log"
REQ_TIMEOUT = 30
SLEEP_BETWEEN = 2.0  # 每个请求间隔秒数，错峰减压
GROUP_FAIL_CIRCUIT = 4  # 单分组连续失败这么多次则熔断该分组

# 非文本对话类模型：探测用的 /v1/chat/completions 对它们无意义或报错
BLACKLIST = (
    "gpt-image", "dall", "midjourney", "suno", "tts", "whisper",
    "stable-diffusion", "flux", "embed", "bge", "m3e", "speech",
    "sora", "video", "realtime",
)


def log(msg):
    try:
        with open(LOG, "a", encoding="utf-8") as f:
            f.write(f"{time.strftime('%F %T')} {msg}\n")
    except Exception:
        pass


def load_keys():
    try:
        with open(KEYS_FILE, encoding="utf-8") as f:
            data = json.load(f)
        return {str(k): str(v) for k, v in data.items() if v}
    except Exception as e:
        log(f"ERROR 读取 {KEYS_FILE} 失败: {e}")
        return {}


def fetch_models():
    try:
        with urllib.request.urlopen(f"{API_BASE}/api/pricing", timeout=10) as r:
            return json.load(r).get("data", [])
    except Exception as e:
        log(f"ERROR 拉取模型清单失败: {e}")
        return None


def probe(model, key):
    body = json.dumps({
        "model": model,
        "messages": [{"role": "user", "content": "hi"}],
        "max_tokens": 1,
        "stream": False,
    }).encode("utf-8")
    req = urllib.request.Request(
        f"{API_BASE}/v1/chat/completions",
        data=body,
        headers={
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=REQ_TIMEOUT) as r:
            return r.status
    except urllib.error.HTTPError as e:
        return e.code
    except Exception:
        return 0


def main():
    keys = load_keys()
    if not keys:
        log("ERROR 无分组 key 配置，跳过")
        sys.exit(1)

    models = fetch_models()
    if models is None:
        sys.exit(1)

    total = ok = fail = skip = 0
    skip_percall = 0
    # 分组级连续失败计数（用于熔断，避免高峰期全失败还猛打）
    group_fail_streak = {g: 0 for g in keys}
    start = time.time()

    for m in models:
        name = m.get("model_name", "")
        if not name:
            continue
        # 按次付费(quota_type==1) 不探测：固定单价、探测即扣费且无 chat 语义
        if m.get("quota_type") == 1:
            skip_percall += 1
            continue
        if any(b in name.lower() for b in BLACKLIST):
            skip += 1
            continue
        for g in m.get("enable_groups", []):
            key = keys.get(g)
            if not key:
                continue
            # 该分组连续失败熔断：跳过剩余模型，高峰期不浪费请求
            if group_fail_streak.get(g, 0) >= GROUP_FAIL_CIRCUIT:
                skip += 1
                continue
            total += 1
            code = probe(name, key)
            if code == 200:
                ok += 1
                group_fail_streak[g] = 0
            else:
                fail += 1
                group_fail_streak[g] = group_fail_streak.get(g, 0) + 1
                log(f"FAIL {name} @ {g} http={code}")
            time.sleep(SLEEP_BETWEEN)  # 错峰减压

    elapsed = int(time.time() - start)
    circuit = [g for g, c in group_fail_streak.items() if c >= GROUP_FAIL_CIRCUIT]
    log(f"probe done total={total} ok={ok} fail={fail} skip={skip} "
        f"skip_percall={skip_percall} elapsed={elapsed}s circuit={circuit}")


if __name__ == "__main__":
    main()
