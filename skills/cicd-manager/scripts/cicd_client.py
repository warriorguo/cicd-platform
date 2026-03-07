#!/usr/bin/env python3
"""
CI/CD Platform API Client

A helper script for interacting with the CI/CD platform API.
Usage: python cicd_client.py <base_url> <command> [args...]

Commands:
  list-apps                              List all applications
  get-app <app_id>                       Get app details
  create-app <name> <git_url>            Create a new app
  delete-app <app_id>                    Delete an app

  build <app_id> <commit_sha> [branch]   Trigger a build
  list-releases <app_id>                 List releases for an app
  get-release <release_id>               Get release details
  build-status <release_id>              Get build status
  build-logs <release_id> [stage]        Get build logs

  deploy <release_id> [cpu_req] [cpu_lim] [mem_req] [mem_lim]  Deploy a release
  rollback <release_id>                  Rollback to a release
  get-deployments <app_id>               Get deployment status
  scale <app_id> <replicas>              Scale a deployment
  delete-deployment <app_id> <rel_id>    Delete a deployment

  pod-logs <app_id> <pod_name>           Get pod runtime logs
  pod-describe <app_id> <pod_name>       Describe a pod

  ingress <app_id>                       Get ingress info
  branches <app_id>                      List git branches
"""

import sys
import json
import urllib.request
import urllib.error
import time


def make_request(base_url, method, endpoint, data=None, headers=None):
    """Make an HTTP request to the API."""
    url = f"{base_url.rstrip('/')}{endpoint}"

    if headers is None:
        headers = {}
    headers['Content-Type'] = 'application/json'

    body = None
    if data is not None:
        body = json.dumps(data).encode('utf-8')

    req = urllib.request.Request(url, data=body, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=30) as response:
            return json.loads(response.read().decode('utf-8'))
    except urllib.error.HTTPError as e:
        error_body = e.read().decode('utf-8')
        try:
            error_json = json.loads(error_body)
            print(f"Error {e.code}: {error_json.get('error', error_body)}", file=sys.stderr)
        except:
            print(f"Error {e.code}: {error_body}", file=sys.stderr)
        sys.exit(1)
    except urllib.error.URLError as e:
        print(f"Connection error: {e.reason}", file=sys.stderr)
        sys.exit(1)


def list_apps(base_url):
    """List all applications."""
    result = make_request(base_url, 'GET', '/api/apps')
    print(json.dumps(result, indent=2))


def get_app(base_url, app_id):
    """Get application details."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}')
    print(json.dumps(result, indent=2))


def create_app(base_url, name, git_url, branch='main', service_port=8080):
    """Create a new application."""
    data = {
        'name': name,
        'git_url': git_url,
        'default_branch': branch,
        'service_port': service_port
    }
    result = make_request(base_url, 'POST', '/api/apps', data)
    print(json.dumps(result, indent=2))
    return result


def delete_app(base_url, app_id):
    """Delete an application."""
    result = make_request(base_url, 'DELETE', f'/api/apps/{app_id}')
    print(json.dumps(result, indent=2))


def trigger_build(base_url, app_id, commit_sha, branch='main'):
    """Trigger a new build."""
    data = {
        'branch': branch,
        'commit_sha': commit_sha
    }
    result = make_request(base_url, 'POST', f'/api/apps/{app_id}/releases', data)
    print(json.dumps(result, indent=2))
    return result


def list_releases(base_url, app_id, page=1, limit=10):
    """List releases for an app."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}/releases?page={page}&limit={limit}')
    print(json.dumps(result, indent=2))


def get_release(base_url, release_id):
    """Get release details."""
    result = make_request(base_url, 'GET', f'/api/releases/{release_id}')
    print(json.dumps(result, indent=2))
    return result


def get_build_status(base_url, release_id):
    """Get build status."""
    result = make_request(base_url, 'GET', f'/api/releases/{release_id}/build-status')
    print(json.dumps(result, indent=2))
    return result


def get_build_logs(base_url, release_id, stage=None):
    """Get build logs."""
    endpoint = f'/api/releases/{release_id}/build-logs'
    if stage:
        endpoint = f'/api/releases/{release_id}/build-logs/{stage}'
    result = make_request(base_url, 'GET', endpoint)
    print(json.dumps(result, indent=2))


def deploy_release(base_url, release_id, env_vars=None, cpu_request=None, cpu_limit=None, memory_request=None, memory_limit=None):
    """Deploy a release."""
    data = {}
    if env_vars:
        data['env_vars'] = env_vars
    if cpu_request:
        data['cpu_request'] = cpu_request
    if cpu_limit:
        data['cpu_limit'] = cpu_limit
    if memory_request:
        data['memory_request'] = memory_request
    if memory_limit:
        data['memory_limit'] = memory_limit
    result = make_request(base_url, 'POST', f'/api/releases/{release_id}/deploy', data)
    print(json.dumps(result, indent=2))
    return result


def rollback_release(base_url, release_id):
    """Rollback to a release."""
    result = make_request(base_url, 'POST', f'/api/releases/{release_id}/rollback', {})
    print(json.dumps(result, indent=2))


def get_deployments(base_url, app_id):
    """Get deployment status."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}/deployments')
    print(json.dumps(result, indent=2))
    return result


def scale_deployment(base_url, app_id, replicas):
    """Scale a deployment."""
    data = {'replicas': int(replicas)}
    result = make_request(base_url, 'POST', f'/api/apps/{app_id}/scale', data)
    print(json.dumps(result, indent=2))


def delete_deployment(base_url, app_id, release_id):
    """Delete a deployment."""
    result = make_request(base_url, 'DELETE', f'/api/apps/{app_id}/deployments/{release_id}')
    print(json.dumps(result, indent=2))


def get_pod_logs(base_url, app_id, pod_name, container=None):
    """Get pod runtime logs."""
    endpoint = f'/api/apps/{app_id}/pods/{pod_name}/logs'
    if container:
        endpoint += f'?container={container}'
    result = make_request(base_url, 'GET', endpoint)
    print(json.dumps(result, indent=2))


def describe_pod(base_url, app_id, pod_name):
    """Describe a pod."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}/pods/{pod_name}/describe')
    print(json.dumps(result, indent=2))


def get_ingress(base_url, app_id):
    """Get ingress information."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}/ingress')
    print(json.dumps(result, indent=2))


def list_branches(base_url, app_id):
    """List git branches."""
    result = make_request(base_url, 'GET', f'/api/apps/{app_id}/branches')
    print(json.dumps(result, indent=2))


def wait_for_build(base_url, release_id, timeout=600, interval=5):
    """Wait for a build to complete."""
    start = time.time()
    while time.time() - start < timeout:
        status = make_request(base_url, 'GET', f'/api/releases/{release_id}/build-status')
        overall = status.get('overall_status', '')

        if overall == 'success':
            print(f"Build completed successfully!")
            return True
        elif overall == 'failed':
            print(f"Build failed!", file=sys.stderr)
            return False

        progress = status.get('progress_percentage', 0)
        current = status.get('current_stage', 'unknown')
        print(f"Build in progress: {progress}% (stage: {current})")

        time.sleep(interval)

    print(f"Build timed out after {timeout} seconds", file=sys.stderr)
    return False


def main():
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)

    base_url = sys.argv[1]
    command = sys.argv[2]
    args = sys.argv[3:]

    commands = {
        'list-apps': lambda: list_apps(base_url),
        'get-app': lambda: get_app(base_url, args[0]),
        'create-app': lambda: create_app(base_url, args[0], args[1], args[2] if len(args) > 2 else 'main'),
        'delete-app': lambda: delete_app(base_url, args[0]),

        'build': lambda: trigger_build(base_url, args[0], args[1], args[2] if len(args) > 2 else 'main'),
        'list-releases': lambda: list_releases(base_url, args[0]),
        'get-release': lambda: get_release(base_url, args[0]),
        'build-status': lambda: get_build_status(base_url, args[0]),
        'build-logs': lambda: get_build_logs(base_url, args[0], args[1] if len(args) > 1 else None),
        'wait-build': lambda: wait_for_build(base_url, args[0]),

        'deploy': lambda: deploy_release(base_url, args[0],
                                         cpu_request=args[1] if len(args) > 1 else None,
                                         cpu_limit=args[2] if len(args) > 2 else None,
                                         memory_request=args[3] if len(args) > 3 else None,
                                         memory_limit=args[4] if len(args) > 4 else None),
        'rollback': lambda: rollback_release(base_url, args[0]),
        'get-deployments': lambda: get_deployments(base_url, args[0]),
        'scale': lambda: scale_deployment(base_url, args[0], args[1]),
        'delete-deployment': lambda: delete_deployment(base_url, args[0], args[1]),

        'pod-logs': lambda: get_pod_logs(base_url, args[0], args[1]),
        'pod-describe': lambda: describe_pod(base_url, args[0], args[1]),

        'ingress': lambda: get_ingress(base_url, args[0]),
        'branches': lambda: list_branches(base_url, args[0]),
    }

    if command not in commands:
        print(f"Unknown command: {command}", file=sys.stderr)
        print(__doc__)
        sys.exit(1)

    try:
        commands[command]()
    except IndexError:
        print(f"Missing required arguments for command: {command}", file=sys.stderr)
        print(__doc__)
        sys.exit(1)


if __name__ == '__main__':
    main()
