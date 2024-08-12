import json
import os
from unittest.mock import patch, mock_open

import context_manager


def get_agents():
    return [
        {"host": "agent1", "roles": ["role1", "role"]},
        {"host": "agent2", "roles": ["role2", "role"]},
        {"host": "agent3", "roles": ["role2", "role"]}
    ]


@patch.dict(os.environ, {'AGENT_IP': 'agent1'})
@patch("context_manager.get_agent_id", return_value="agent_id")
@patch("context_manager.dotenv_values", return_value={"CLUSTER_ID": "cluster_id"})
def test_load_values_agent1(mock_dotenv, mock_get_agent_id):
    agents = get_agents()

    with patch('builtins.open', mock_open(read_data=json.dumps(agents))) as mock_file:
        values = context_manager.get_values()

    assert values == {'AGENT_0': 'agent1',
                      'AGENT_1': 'agent2',
                      'AGENT_2': 'agent3',
                      'AGENT_ALLOCATION_INDEX': 0,
                      'AGENT_ID': 'agent_id',
                      'AGENT_IP': 'agent1',
                      'AGENT_LENGTH': 3,
                      'AGENT_NODES': 'agent1,agent2,agent3',
                      'AGENT_OTHERS': 'agent2,agent3',
                      'CLUSTER_ID': 'cluster_id',
                      'ROLE1_0': 'agent1',
                      'ROLE1_ALLOCATION_INDEX': 0,
                      'ROLE1_LENGTH': 1,
                      'ROLE1_NODES': 'agent1',
                      'ROLE1_OTHERS': '',
                      'ROLE2_0': 'agent2',
                      'ROLE2_1': 'agent3',
                      'ROLE2_LENGTH': 2,
                      'ROLE2_NODES': 'agent2,agent3',
                      'ROLE2_OTHERS': 'agent2,agent3',
                      'ROLES': 'agent,role,role1',
                      'ROLE_0': 'agent1',
                      'ROLE_1': 'agent2',
                      'ROLE_2': 'agent3',
                      'ROLE_ALLOCATION_INDEX': 0,
                      'ROLE_LENGTH': 3,
                      'ROLE_NODES': 'agent1,agent2,agent3',
                      'ROLE_OTHERS': 'agent2,agent3'
                      }


@patch.dict(os.environ, {'AGENT_IP': 'agent2'})
@patch("context_manager.get_agent_id", return_value="agent_id")
@patch("context_manager.dotenv_values", return_value={"CLUSTER_ID": "cluster_id"})
def test_load_values_agent2(mock_dotenv, mock_get_agent_id):
    agents = get_agents()

    with patch('builtins.open', mock_open(read_data=json.dumps(agents))) as mock_file:
        values = context_manager.get_values()

    assert values == {'AGENT_0': 'agent1',
                      'AGENT_1': 'agent2',
                      'AGENT_2': 'agent3',
                      'AGENT_ALLOCATION_INDEX': 1,
                      'AGENT_ID': 'agent_id',
                      'AGENT_IP': 'agent2',
                      'AGENT_LENGTH': 3,
                      'AGENT_NODES': 'agent1,agent2,agent3',
                      'AGENT_OTHERS': 'agent1,agent3',
                      'CLUSTER_ID': 'cluster_id',
                      'ROLE1_0': 'agent1',
                      'ROLE1_LENGTH': 1,
                      'ROLE1_NODES': 'agent1',
                      'ROLE1_OTHERS': 'agent1',
                      'ROLE2_0': 'agent2',
                      'ROLE2_1': 'agent3',
                      'ROLE2_ALLOCATION_INDEX': 0,
                      'ROLE2_LENGTH': 2,
                      'ROLE2_NODES': 'agent2,agent3',
                      'ROLE2_OTHERS': 'agent3',
                      'ROLES': 'agent,role,role2',
                      'ROLE_0': 'agent1',
                      'ROLE_1': 'agent2',
                      'ROLE_2': 'agent3',
                      'ROLE_ALLOCATION_INDEX': 1,
                      'ROLE_LENGTH': 3,
                      'ROLE_NODES': 'agent1,agent2,agent3',
                      'ROLE_OTHERS': 'agent1,agent3'}


@patch.dict(os.environ, {'AGENT_IP': 'agent3'})
@patch("context_manager.get_agent_id", return_value="agent_id")
@patch("context_manager.dotenv_values", return_value={"CLUSTER_ID": "cluster_id"})
def test_load_values_agent3(mock_dotenv, mock_get_agent_id):
    agents = get_agents()

    with patch('builtins.open', mock_open(read_data=json.dumps(agents))) as mock_file:
        values = context_manager.get_values()

    assert values == {'AGENT_0': 'agent1',
                      'AGENT_1': 'agent2',
                      'AGENT_2': 'agent3',
                      'AGENT_ALLOCATION_INDEX': 2,
                      'AGENT_ID': 'agent_id',
                      'AGENT_IP': 'agent3',
                      'AGENT_LENGTH': 3,
                      'AGENT_NODES': 'agent1,agent2,agent3',
                      'AGENT_OTHERS': 'agent1,agent2',
                      'CLUSTER_ID': 'cluster_id',
                      'ROLE1_0': 'agent1',
                      'ROLE1_LENGTH': 1,
                      'ROLE1_NODES': 'agent1',
                      'ROLE1_OTHERS': 'agent1',
                      'ROLE2_0': 'agent2',
                      'ROLE2_1': 'agent3',
                      'ROLE2_ALLOCATION_INDEX': 1,
                      'ROLE2_LENGTH': 2,
                      'ROLE2_NODES': 'agent2,agent3',
                      'ROLE2_OTHERS': 'agent2',
                      'ROLES': 'agent,role,role2',
                      'ROLE_0': 'agent1',
                      'ROLE_1': 'agent2',
                      'ROLE_2': 'agent3',
                      'ROLE_ALLOCATION_INDEX': 2,
                      'ROLE_LENGTH': 3,
                      'ROLE_NODES': 'agent1,agent2,agent3',
                      'ROLE_OTHERS': 'agent1,agent2'}


def test_get_agent_id():
    with patch('builtins.open', mock_open(read_data="agent_id")) as mock_file:
        assert context_manager.get_agent_id() == "agent_id"


def test_get_cluster_id():
    with patch('builtins.open', mock_open(read_data="cluster_id")) as mock_file:
        assert context_manager.get_agent_id() == "cluster_id"


@patch("context_manager.dotenv_values", return_value={"SECRET": "1"})
def test_load_secrets(mock_dotenv):
    assert context_manager.load_secrets({}) == {'SECRET': '1'}
    assert context_manager.load_secrets({'VALUE': 1}) == {'SECRET': '1', 'VALUE': 1}
