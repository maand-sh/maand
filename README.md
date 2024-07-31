**Maand** is a job orchestrator framework designed to manage and distribute jobs across multiple agents. Here's how it works:

1. **Agent Configuration**:
    - Agents are defined in an `agents.json` file located in the workspace folder.
    - Each agent entry includes a `host` and a list of `roles`.
    ```json
    [
      {
        "host": "192.168.1.110", "roles": ["opensearch"]
      },
      {
        "host": "192.168.1.119", "roles": ["opensearch"]
      },
      {
        "host": "192.168.1.134", "roles": ["opensearch"]
      }
    ]
    ```

2. **Job Matching**:
    - Jobs are stored in the `jobs` folder within the workspace.
    - Each job has a manifest that specifies the roles required.
    - Maand matches jobs to agents based on these roles. If an agent's roles match the roles required by a job, that job is assigned to the agent.

3. **Job Distribution**:
    - The selected jobs are copied to the agents.
    - Files are synchronized to the `/opt/agent` directory on each agent's VM using `rsync`.
