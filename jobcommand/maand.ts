// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

/** Client for the maand command runtime API (KV store, demands, semaphores). */

const RUNTIME_API_PORT = 8080;
const ROUTE_STORE_KEYS = "/kv";
const ROUTE_STORE_KEYS_LIST = "/kv/keys";
const ROUTE_STORE_SECRET = "/kv/secret";
const ROUTE_DEMANDS = "/demands";
const ROUTE_SEMAPHORE_ACQUIRE = "/semaphore/acquire";
const ROUTE_SEMAPHORE_RELEASE = "/semaphore/release";
const ROUTE_SEMAPHORE_STATUS = "/semaphore/status";

export function allocationId(): string | undefined {
  return process.env.ALLOCATION_ID;
}

export function allocationIp(): string | undefined {
  return process.env.ALLOCATION_IP;
}

export function isAllocationDisabled(): boolean {
  return process.env.DISABLED === "1";
}

export function commandEvent(): string | undefined {
  return process.env.EVENT;
}

export function commandName(): string | undefined {
  return process.env.COMMAND;
}

export function jobName(): string | undefined {
  return process.env.JOB;
}

function runtimeApiBaseUrl(): string {
  const host = process.env.JOB_COMMAND_API_HOST ?? "0.0.0.0";
  return `http://${host}:${RUNTIME_API_PORT}`;
}

function runtimeRequestHeaders(): Record<string, string> {
  return {
    "X-ALLOCATION-ID": allocationId() ?? "",
    COMMAND: commandName() ?? "",
    EVENT: commandEvent() ?? "",
  };
}

export async function getStoreValue(namespace: string, key: string): Promise<Response> {
  // Matches Python requests.get(..., json=...) — body on GET is required by the runtime API.
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_KEYS}`, {
    method: "GET",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ namespace, key }),
  } as RequestInit);
}

export async function putJobVariable(key: string, value: string): Promise<Response> {
  const job = jobName();
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_KEYS}`, {
    method: "PUT",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      namespace: `vars/job/${job}`,
      key,
      value,
    }),
  });
}

export async function putJobSecret(key: string, value: string): Promise<Response> {
  const job = jobName();
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_SECRET}`, {
    method: "PUT",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      namespace: `secrets/job/${job}`,
      key,
      value,
    }),
  });
}

export async function listJobKeys(namespace?: string): Promise<Response> {
  const body: Record<string, string> = {};
  if (namespace) {
    body.namespace = namespace;
  }
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_KEYS_LIST}`, {
    method: "GET",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify(body),
  } as RequestInit);
}

export async function deleteJobVariable(key: string): Promise<Response> {
  const job = jobName();
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_KEYS}`, {
    method: "DELETE",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      namespace: `vars/job/${job}`,
      key,
    }),
  });
}

export async function deleteJobSecret(key: string): Promise<Response> {
  const job = jobName();
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_STORE_SECRET}`, {
    method: "DELETE",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      namespace: `secrets/job/${job}`,
      key,
    }),
  });
}

export async function listCommandDemands(): Promise<Response> {
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_DEMANDS}`, {
    headers: runtimeRequestHeaders(),
  });
}

export async function acquireSemaphore(
  name: string,
  capacity = 1,
  timeoutSeconds = 600,
): Promise<Response> {
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_SEMAPHORE_ACQUIRE}`, {
    method: "POST",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ name, capacity, timeout_seconds: timeoutSeconds }),
    signal: AbortSignal.timeout((timeoutSeconds + 30) * 1000),
  });
}

export async function releaseSemaphore(name: string): Promise<Response> {
  return fetch(`${runtimeApiBaseUrl()}${ROUTE_SEMAPHORE_RELEASE}`, {
    method: "POST",
    headers: {
      ...runtimeRequestHeaders(),
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ name }),
  });
}

export async function semaphoreStatus(name: string): Promise<Response> {
  const url = new URL(`${runtimeApiBaseUrl()}${ROUTE_SEMAPHORE_STATUS}`);
  url.searchParams.set("name", name);
  return fetch(url, { headers: runtimeRequestHeaders() });
}

// Backward-compatible aliases for older job command scripts.
export const getAllocationId = allocationId;
export const getAllocationIp = allocationIp;
export const getEvent = commandEvent;
export const getCommand = commandName;
export const getJob = jobName;
export const kvGet = getStoreValue;
export const kvPut = putJobVariable;
export const kvPutSecret = putJobSecret;
export const getDemands = listCommandDemands;
