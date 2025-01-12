import socket
import time


def wait_for_host(target_host, target_port=22):
    max_timeout = 120
    start_time = time.time()

    while True:
        try:
            target_ip = socket.gethostbyname(target_host)
            with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
                s.settimeout(1)
                s.connect((target_ip, target_port))

            print(f"Machine is up and reachable on {target_host}:{target_port}", flush=True)
            break
        except (socket.timeout, ConnectionRefusedError, OSError) as e:
            elapsed_time = time.time() - start_time
            if elapsed_time >= max_timeout:
                print(f"Timeout reached. {target_host}:{target_port} is not reachable.", flush=True)
                exit(1)

            if isinstance(e, OSError) and e.errno == 64:
                print(f"{target_host} is down or unreachable. Retrying...", flush=True)
            else:
                print(f"Waiting for {target_host}:{target_port} to become reachable...", flush=True)
            time.sleep(1)
