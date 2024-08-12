import json
from unittest.mock import patch, mock_open

import utils


def test_get_agents():
    agent_json = [{"host": "localhost30", "roles": ["role1", "role2"], "tags": {"foo": "bar"}},
                  {"host": "localhost1", "roles": ["role1", "role2"]}]
    with patch('builtins.open', mock_open(read_data=json.dumps(agent_json))) as mock_file:
        agents = utils.get_agents()

    keys = list(agents.keys())
    assert keys[0] == "localhost30"
    assert keys[1] == "localhost1"
    assert agents["localhost30"]["roles"] == ["role1", "role2", "agent"]
    assert agents["localhost1"]["roles"] == ["role1", "role2", "agent"]

    assert agents["localhost30"]["tags"] == {"foo": "bar"}
    assert agents["localhost1"]["tags"] == {}


def test_get_agents_no_roles():
    agent_json = [{"host": "localhost1"},
                  {"host": "localhost2", "roles": ["role1", "role2"]}]
    with patch('builtins.open', mock_open(read_data=json.dumps(agent_json))) as mock_file:
        agents = utils.get_agents()

    keys = list(agents.keys())
    assert keys[0] == "localhost1"
    assert keys[1] == "localhost2"
    assert agents["localhost1"]["roles"] == ["agent"]
    assert agents["localhost2"]["roles"] == ["role1", "role2", "agent"]


def test_get_agents_role_filter():
    agent_json = [{"host": "localhost1", "roles": ["role1", "role2"]},
                  {"host": "localhost2", "roles": ["role2", "role3"]}]

    with patch('builtins.open', mock_open(read_data=json.dumps(agent_json))) as mock_file:
        agents = utils.get_agents(["role1"])
        keys = list(agents.keys())
        assert len(keys) == 1
        assert keys[0] == "localhost1"

        agents = utils.get_agents(["role4"])
        keys = list(agents.keys())
        assert len(keys) == 0


def test_get_agents_and_roles():
    agent_json = [{"host": "localhost3", "roles": ["role1", "role2"], "tags": {"foo": "bar"}},
                  {"host": "localhost4", "roles": ["role1", "role2"]}]
    with patch('builtins.open', mock_open(read_data=json.dumps(agent_json))) as mock_file:
        agents = utils.get_agent_and_roles(["role1", "role2"])

    keys = list(agents.keys())
    assert keys[0] == "localhost3"
    assert keys[1] == "localhost4"
    assert agents["localhost3"] == ["role1", "role2", "agent"]
    assert agents["localhost4"] == ["role1", "role2", "agent"]


def test_get_agents_and_tags():
    agent_json = [{"host": "localhost3", "roles": ["role1", "role2"], "tags": {"foo": "bar"}},
                  {"host": "localhost4", "roles": ["role1", "role2"]}]
    with patch('builtins.open', mock_open(read_data=json.dumps(agent_json))) as mock_file:
        agents = utils.get_agent_and_tags(["role1", "role2"])

    keys = list(agents.keys())
    assert keys[0] == "localhost3"
    assert keys[1] == "localhost4"
    assert agents["localhost3"] == {"foo": "bar"}
    assert agents["localhost4"] == {}


@patch('os.path.exists', return_value=True)
def test_get_job_metadata(mock_os_path_exists):
    with patch('builtins.open', mock_open(read_data=json.dumps({"roles": []}))) as mock_file:
        metadata = utils.get_job_metadata("job1")
        assert metadata == {"roles": []}
        assert mock_file.call_count == 1


@patch('os.path.exists', return_value=False)
def test_get_job_metadata_not_exists(mock_os_path_exists):
    with patch('builtins.open', mock_open(read_data=json.dumps({"roles": []}))) as mock_file:
        metadata = utils.get_job_metadata("job1")
        assert metadata == {"roles": []}


@patch('os.path.exists', return_value=False)
def test_get_role_and_jobs_no_roles(mock_os_path_exists):
    with patch('glob.glob', return_value=["path1/manifest.json", "path2/manifest.json"]) as mock_glob:
        role_and_jobs = utils.get_role_and_jobs()
        assert role_and_jobs == {}


def mock_open_side_effect(filepath, *args, **kwargs):
    if filepath == "/workspace/jobs/path1/manifest.json":
        return mock_open(read_data=json.dumps({"roles": ["role1"]})).return_value
    elif filepath == "/workspace/jobs/path2/manifest.json":
        return mock_open(read_data=json.dumps({"roles": ["role2"]})).return_value
    elif filepath == "/workspace/jobs/path3/manifest.json":
        return mock_open(read_data=json.dumps({"roles": ["role3"]})).return_value
    elif filepath == "/workspace/jobs/path4/manifest.json":
        return mock_open(read_data=json.dumps({"roles": ["role3"]})).return_value
    else:
        raise FileNotFoundError


@patch('os.path.exists', return_value=True)
def test_get_role_and_jobs(mock_os_path_exists):
    with patch('glob.glob', return_value=["path1/manifest.json", "path2/manifest.json"]):
        with patch('builtins.open', side_effect=mock_open_side_effect) as mock_file:
            role_and_jobs = utils.get_role_and_jobs()
            assert role_and_jobs == {'role1': ['path1'], 'role2': ['path2']}

    with patch('glob.glob', return_value=["path3/manifest.json", "path4/manifest.json"]):
        with patch('builtins.open', side_effect=mock_open_side_effect) as mock_file:
            role_and_jobs = utils.get_role_and_jobs()
            assert role_and_jobs == {'role3': ['path3', 'path4']}


def test_get_logger():
    logger1 = utils.get_logger()
    logger2 = utils.get_logger()

    assert logger1 == logger2
