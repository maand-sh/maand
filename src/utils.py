import configparser
import re
import subprocess
from functools import cache

import const
from log_manager import LoggerManager

log_manager = LoggerManager()


def get_logger(ns="maand"):
    return log_manager.get_logger(ns)


@cache
def get_maand_conf():
    config_parser = configparser.ConfigParser()
    config_parser.read(const.CONF_PATH)
    return config_parser


def get_maand_jobs_conf():
    jobs_conf_path = "workspace/maand.jobs.conf"
    maand_conf = get_maand_conf()
    if maand_conf.has_option("default", "jobs_conf_path"):
        jobs_conf_path = maand_conf.get("default", "jobs_conf_path")
    return jobs_conf_path


def split_list(input_list, chunk_size=3):
    return [
        input_list[i: i + chunk_size] for i in range(0, len(input_list), chunk_size)
    ]


def extract_size_in_mb(size_string):
    unit_to_mb = {
        "MB": 1,
        "GB": 1024,
        "TB": 1024 ** 2,
    }

    if isinstance(size_string, (int, float)):
        return float(size_string)
    if isinstance(size_string, str) and size_string.strip().isdigit():
        return float(size_string)

    match = re.match(r"([\d.]+)\s*([a-zA-Z]*)", size_string)
    if not match:
        raise ValueError(f"Invalid size input: {size_string}")

    size = float(match.group(1))
    unit = match.group(2).upper() if match.group(2) else "MB"  # Default to MB if no unit is provided

    if unit not in unit_to_mb:
        raise ValueError(f"Unit smaller than MB or invalid: {unit}")

    size_in_mb = size * unit_to_mb[unit]
    return size_in_mb


def extract_cpu_frequency_in_mhz(freq_string):
    unit_to_mhz = {
        "MHZ": 1,  # Megahertz
        "GHZ": 10 ** 3,  # Gigahertz to MHz
        "THZ": 10 ** 6,  # Terahertz to MHz
    }

    if isinstance(freq_string, (int, float)):
        return float(freq_string)
    if isinstance(freq_string, str) and freq_string.strip().isdigit():
        return float(freq_string)

    match = re.match(r"([\d.]+)\s*([a-zA-Z]+)", freq_string)
    if not match:
        raise ValueError(f"Invalid frequency string format: '{freq_string}'")

    frequency = float(match.group(1))
    unit = match.group(2).upper()

    if unit not in unit_to_mhz:
        raise ValueError(f"Unsupported or invalid unit: '{unit}' (unit must be MHz or larger)")

    frequency_in_mhz = frequency * unit_to_mhz[unit]
    return frequency_in_mhz


def stop_the_world():
    subprocess.run(["kill", "-TERM", "1"])
