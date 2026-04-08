from __future__ import annotations

import argparse
import json
import sys

from .agent import Agent
from .config import Config
from .crypto import generate_and_write_key_pair


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="ssnp-agent")
    parser.add_argument("--config", required=True, help="path to config yaml")
    subparsers = parser.add_subparsers(dest="command")

    gen_key = subparsers.add_parser("gen-key")
    gen_key.add_argument("--out-dir", default="./keys")

    enroll = subparsers.add_parser("enroll")
    enroll.add_argument("--challenge-id", required=True)

    run = subparsers.add_parser("run")
    run.set_defaults(no_extra_args=True)

    check = subparsers.add_parser("check")
    check.add_argument("--event-type", required=True)
    check.add_argument("--event-id", required=True)

    telemetry = subparsers.add_parser("telemetry")
    telemetry.add_argument("--warning-flag", action="append", default=[])

    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if not args.command:
        parser.error("missing command: run | enroll | check | telemetry | gen-key")
    try:
        if args.command == "gen-key":
            private_key_path, public_key_path = generate_and_write_key_pair(args.out_dir)
            json.dump(
                {
                    "private_key_path": private_key_path,
                    "public_key_path": public_key_path,
                },
                sys.stdout,
            )
            sys.stdout.write("\n")
            return 0

        cfg = Config.load(args.config)
        agent = Agent.from_config(cfg)
        if args.command == "enroll":
            agent.enroll(args.challenge_id)
            return 0
        if args.command == "run":
            agent.run()
            return 0
        if args.command == "check":
            agent.run_checks(args.event_type, args.event_id)
            return 0
        if args.command == "telemetry":
            if not args.warning_flag:
                raise ValueError("missing --warning-flag")
            agent.submit_telemetry(args.warning_flag)
            return 0
        raise ValueError(f"unknown command: {args.command}")
    except Exception as err:  # noqa: BLE001
        print(f"program-agent: {err}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
