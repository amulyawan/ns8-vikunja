*** Settings ***
Library    SSHLibrary

*** Variables ***
${HOST}    vikunja.example.org

*** Test Cases ***
Check if vikunja is installed correctly
    ${output}  ${rc} =    Execute Command    add-module ${IMAGE_URL} 1
    ...    return_rc=True
    Should Be Equal As Integers    ${rc}  0
    &{output} =    Evaluate    ${output}
    Set Suite Variable    ${module_id}    ${output.module_id}

Check if vikunja can be configured
    ${rc} =    Execute Command    api-cli run module/${module_id}/configure-module --data '{"host":"${HOST}","http2https":true,"lets_encrypt":false}'
    ...    return_rc=True  return_stdout=False
    Should Be Equal As Integers    ${rc}  0

Check if vikunja works as expected
    ${rc} =    Execute Command    bash -c 'for i in $(seq 1 60); do curl -k -s -o /dev/null -f --resolve ${HOST}:443:127.0.0.1 https://${HOST}/ && exit 0; sleep 5; done; exit 1'
    ...    return_rc=True  return_stdout=False
    Should Be Equal As Integers    ${rc}  0

Check API version and database health
    ${output}  ${rc} =    Execute Command    runagent -m ${module_id} bash -c 'curl -fsS "http://127.0.0.1:$TCP_PORT/api/v2/info"'
    ...    return_rc=True
    Should Be Equal As Integers    ${rc}  0
    ${info} =    Evaluate    json.loads($output)    modules=json
    ${version} =    Evaluate    $info['version'].lstrip('v')
    Should Be Equal    ${version}    2.8.0
    ${output}  ${rc} =    Execute Command    runagent -m ${module_id} bash -c 'curl -fsS "http://127.0.0.1:$TCP_PORT/api/v2/health"'
    ...    return_rc=True
    Should Be Equal As Integers    ${rc}  0
    ${health} =    Evaluate    json.loads($output)    modules=json
    Should Be Equal    ${health}[status]    OK

Check MCP rejects anonymous requests
    ${output}  ${rc} =    Execute Command    runagent -m ${module_id} bash -c 'curl -sS -D - -o /dev/null -H "Content-Type: application/json" --data "{}" "http://127.0.0.1:$TCP_PORT/api/v2/mcp"'
    ...    return_rc=True
    Should Be Equal As Integers    ${rc}  0
    Should Match Regexp    ${output}    ^HTTP/[0-9.]+ (401|403)

Check if vikunja is removed correctly
    ${rc} =    Execute Command    remove-module --no-preserve ${module_id}
    ...    return_rc=True  return_stdout=False
    Should Be Equal As Integers    ${rc}  0
