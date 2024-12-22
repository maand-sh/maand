import logging
import os
from threading import Lock


class LoggerManager:
    _instance = None
    _lock = Lock()

    def __new__(cls, *args, **kwargs):
        with cls._lock:
            if cls._instance is None:
                cls._instance = super(LoggerManager, cls).__new__(cls)
        return cls._instance

    def __init__(self, log_dir="/bucket/logs"):
        if not hasattr(self, "_initialized"):  # Avoid reinitialization
            self._instances = {}
            self._log_dir = log_dir
            os.makedirs(self._log_dir, exist_ok=True)
            self._initialized = True

    def _create_logger(self, ns):
        logger = logging.getLogger(ns)

        if logger.hasHandlers():
            return logger

        console_handler = logging.StreamHandler()
        file_handler = logging.FileHandler(f"{self._log_dir}/{ns}.log")
        maand_handler = logging.FileHandler(f"{self._log_dir}/maand.log")

        console_formatter = logging.Formatter('%(asctime)s - %(name)s - %(levelname)s - %(message)s')
        file_formatter = logging.Formatter('%(message)s')

        console_handler.setFormatter(console_formatter)
        file_handler.setFormatter(file_formatter)
        maand_handler.setFormatter(console_formatter)

        logger.setLevel(logging.INFO)
        file_handler.setLevel(logging.INFO)
        maand_handler.setLevel(logging.INFO)
        console_handler.setLevel(logging.INFO)

        logger.addHandler(console_handler)
        logger.addHandler(file_handler)
        logger.addHandler(maand_handler)

        return logger

    def get_logger(self, ns):
        with self._lock:
            if ns not in self._instances:
                self._instances[ns] = self._create_logger(ns)
            return self._instances[ns]


# Create a single global instance
log_manager = LoggerManager()

def get_log_manager():
    return log_manager
