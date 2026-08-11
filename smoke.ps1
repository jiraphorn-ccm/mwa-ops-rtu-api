$ErrorActionPreference = "Continue"
$base = "http://127.0.0.1:5020"
$api  = "$base/api/rtu/v1"
$run  = Get-Date -Format "MMddHHmmss"

function Call([string]$method, [string]$url, $body) {
    $params = @{ Method = $method; Uri = $url }
    if ($body) {
        $params.Body = ($body | ConvertTo-Json -Depth 8)
        $params.ContentType = "application/json"
    }
    try {
        $res = Invoke-WebRequest @params -UseBasicParsing
        return @{ Status = $res.StatusCode; Body = ($res.Content | ConvertFrom-Json) }
    } catch {
        $status = -1
        if ($_.Exception.Response) { $status = [int]$_.Exception.Response.StatusCode }
        $raw = $_.ErrorDetails.Message
        $parsed = $null
        if ($raw) { try { $parsed = $raw | ConvertFrom-Json } catch { $parsed = @{ message = $raw } } }
        if (-not $parsed) { $parsed = @{ message = $_.Exception.Message } }
        return @{ Status = $status; Body = $parsed }
    }
}

function Show([string]$label, $r) {
    $code = "-"
    if ($r.Body.code) { $code = $r.Body.code }
    Write-Host ("{0,-46} {1,-4} {2,-10} {3}" -f $label, $r.Status, $code, $r.Body.message)
}

Write-Host "--- health ---"
$r = Call GET "$base/health" $null
Write-Host ("{0,-46} {1,-4} {2}" -f "GET /health", $r.Status, $r.Body.status)
$r = Call GET "$base/" $null
Write-Host ("{0,-46} {1,-4} {2}" -f "GET /", $r.Status, $r.Body.name)

Write-Host "`n--- panels ---"
$panelBody = @{ code = "PNL-$run"; location = "Pump Station 1"; latitude = 13.8622; longitude = 100.5601 }
$r = Call POST "$api/panels" $panelBody
Show "POST /panels" $r
$panelId = $r.Body.data.id
Write-Host "   panel id: $panelId  lat=$($r.Body.data.latitude)"

$r = Call POST "$api/panels" @{ code = "PNL-$run" }
Show "POST /panels (duplicate code)" $r

$r = Call POST "$api/panels" @{ code = "PNL-$run-B"; latitude = 999 }
Show "POST /panels (bad latitude)" $r
Write-Host "   errors: $($r.Body.errors | ConvertTo-Json -Compress)"

$r = Call POST "$api/panels" @{ location = "no code" }
Show "POST /panels (missing code)" $r
Write-Host "   errors: $($r.Body.errors | ConvertTo-Json -Compress)"

$r = Call POST "$api/panels" @{ code = "PNL-$run-C"; nope = "x" }
Show "POST /panels (unknown field)" $r

$r = Call GET "$api/panels?limit=5&sort=code&order=ASC" $null
Show "GET /panels" $r
Write-Host "   meta: $($r.Body.data.meta | ConvertTo-Json -Compress)"

$r = Call GET "$api/panels?sort=hax" $null
Show "GET /panels (bad sort)" $r
Write-Host "   errors: $($r.Body.errors | ConvertTo-Json -Compress)"

$r = Call GET "$api/panels/code/PNL-$run" $null
Show "GET /panels/code/PNL-$run" $r
$r = Call GET "$api/panels/not-a-uuid" $null
Show "GET /panels/(bad uuid)" $r
$r = Call GET "$api/panels/00000000-0000-0000-0000-000000000000" $null
Show "GET /panels/(unknown)" $r

$r = Call PATCH "$api/panels/$panelId" @{ location = $null }
Show "PATCH /panels (null clears location)" $r
$loc = "<null>"
if ($null -ne $r.Body.data.location) { $loc = $r.Body.data.location }
Write-Host "   location now: $loc"

$r = Call PATCH "$api/panels/$panelId" @{ code = $null }
Show "PATCH /panels (null on NOT NULL)" $r

$r = Call PATCH "$api/panels/$panelId" @{ location = "Building A" }
Show "PATCH /panels (set location)" $r
Write-Host "   code still: $($r.Body.data.code), location: $($r.Body.data.location)"

Write-Host "`n--- device models ---"
$r = Call POST "$api/device-models" @{ code = "DM-$run"; name = "RTU Controller"; manufacturer = "Siemens"; model = "S7-1200" }
Show "POST /device-models" $r
$modelId = $r.Body.data.id

Write-Host "`n--- panel devices ---"
$deviceBody = @{ device_model_id = $modelId; tag_name = "FT-101"; serial_number = "SN-$run"; installed_at = "2025-01-15"; communication_status = "ONLINE"; health_status = "NORMAL" }
$r = Call POST "$api/panels/$panelId/devices" $deviceBody
Show "POST /panels/(id)/devices" $r
$deviceId = $r.Body.data.id
Write-Host "   panel_code=$($r.Body.data.panel_code) model=$($r.Body.data.device_model_name) calibrations=$($r.Body.data.calibration_count)"

$r = Call POST "$api/panel-devices" @{ panel_id = $panelId; device_model_id = $modelId; tag_name = "FT-101" }
Show "POST /panel-devices (dup tag in panel)" $r

$r = Call POST "$api/panel-devices" @{ panel_id = $panelId; device_model_id = $modelId; serial_number = "SN-$run" }
Show "POST /panel-devices (dup serial)" $r

$r = Call POST "$api/panel-devices" @{ panel_id = "00000000-0000-0000-0000-000000000000"; device_model_id = $modelId }
Show "POST /panel-devices (unknown panel)" $r

$r = Call POST "$api/panel-devices" @{ panel_id = $panelId; device_model_id = $modelId; communication_status = "EXPLODED" }
Show "POST /panel-devices (bad enum)" $r

$r = Call POST "$api/panel-devices/$deviceId/status" @{ communication_status = "DEGRADED"; health_status = "WARNING" }
Show "POST /panel-devices/(id)/status" $r
Write-Host "   comm=$($r.Body.data.communication_status) health=$($r.Body.data.health_status) last_seen=$($r.Body.data.last_seen_at)"

$r = Call GET "$api/panel-devices?health_status=WARNING" $null
Show "GET /panel-devices?health_status=WARNING" $r
Write-Host "   total: $($r.Body.data.meta.total)"

Write-Host "`n--- instruments ---"
$r = Call POST "$api/calibration-instruments" @{ name = "Fluke 754"; manufacturer = "Fluke"; serial_number = "INS-$run"; calibration_date = "2026-01-10"; expire_date = "2027-01-10" }
Show "POST /calibration-instruments" $r
$instId = $r.Body.data.id

$r = Call POST "$api/calibration-instruments" @{ name = "Bad dates"; calibration_date = "2026-05-01"; expire_date = "2026-01-01" }
Show "POST instruments (expire before cal)" $r

$r = Call POST "$api/calibration-instruments" @{ name = "Expired"; serial_number = "INS-$run-OLD"; calibration_date = "2020-01-01"; expire_date = "2021-01-01" }
Show "POST instruments (already expired)" $r
$expiredId = $r.Body.data.id

$r = Call GET "$api/calibration-instruments?expired=true" $null
Show "GET instruments?expired=true" $r
Write-Host "   total=$($r.Body.data.meta.total) is_expired=$($r.Body.data.items[0].is_expired) days=$($r.Body.data.items[0].days_until_expiry)"

Write-Host "`n--- calibrations ---"
$calBody = @{
    panel_device_id = $deviceId
    instrument_id   = $instId
    performed_by    = "Somchai"
    performed_at    = "2026-08-01T09:30:00+07:00"
    result          = "PASS"
    remark          = "Annual cycle"
    readings        = @(
        @{ item_label = "Zero point"; parameter_key = "pressure"; value = 0.02; unit = "bar" },
        @{ item_label = "Span point"; parameter_key = "pressure"; value = 9.98; unit = "bar" },
        @{ item_label = "Mid point";  parameter_key = "pressure"; value = 5.01; unit = "bar" }
    )
}
$r = Call POST "$api/calibrations" $calBody
Show "POST /calibrations (+3 readings)" $r
$calId = $r.Body.data.id
Write-Host "   readings=$($r.Body.data.reading_count) seqs=$($r.Body.data.readings.sequence -join ',') values=$($r.Body.data.readings.value -join ',')"
Write-Host "   panel_code=$($r.Body.data.panel_code) instrument=$($r.Body.data.instrument_name)"

$r = Call POST "$api/calibrations" @{ panel_device_id = $deviceId; instrument_id = $instId; performed_at = "2030-01-01T00:00:00Z"; result = "PASS" }
Show "POST /calibrations (future)" $r

$r = Call POST "$api/calibrations" @{ panel_device_id = $deviceId; instrument_id = $expiredId; performed_at = "2026-08-01T09:30:00+07:00"; result = "PASS" }
Show "POST /calibrations (expired instrument)" $r

$r = Call POST "$api/calibrations" @{ panel_device_id = $deviceId; instrument_id = $instId; performed_at = "2026-08-01T09:30:00+07:00"; result = "MAYBE" }
Show "POST /calibrations (bad result)" $r

$dupBody = @{
    panel_device_id = $deviceId; instrument_id = $instId; performed_at = "2026-08-01T09:30:00+07:00"; result = "PASS"
    readings = @( @{ sequence = 1; parameter_key = "a" }, @{ sequence = 1; parameter_key = "b" } )
}
$r = Call POST "$api/calibrations" $dupBody
Show "POST /calibrations (dup sequence)" $r

$r = Call GET "$api/calibrations/$calId" $null
Show "GET /calibrations/(id)" $r
Write-Host "   readings returned: $($r.Body.data.readings.Count)"

$r = Call PUT "$api/calibrations/$calId/readings" @{ readings = @( @{ parameter_key = "temperature"; value = 25.5; unit = "C" } ) }
Show "PUT /calibrations/(id)/readings" $r
Write-Host "   items now: $($r.Body.data.items.Count) key=$($r.Body.data.items[0].parameter_key)"

$r = Call POST "$api/calibrations/$calId/readings" @{ parameter_key = "humidity"; value = 60; unit = "%" }
Show "POST /calibrations/(id)/readings" $r
Write-Host "   auto sequence: $($r.Body.data.sequence)"

$r = Call GET "$api/calibrations/summary" $null
Show "GET /calibrations/summary" $r
Write-Host "   by_result: $($r.Body.data.by_result | ConvertTo-Json -Compress)"

$r = Call GET "$api/panel-devices/$deviceId" $null
Show "GET /panel-devices/(id)" $r
Write-Host "   calibration_count=$($r.Body.data.calibration_count) last=$($r.Body.data.last_calibrated_at) result=$($r.Body.data.last_calibration_result)"

Write-Host "`n--- deletes ---"
$r = Call DELETE "$api/panels/$panelId/permanent" $null
Show "DELETE /panels/(id)/permanent (in use)" $r

$r = Call DELETE "$api/panel-devices/$deviceId" $null
Show "DELETE /panel-devices/(id) (soft)" $r
Write-Host "   data: $($r.Body.data | ConvertTo-Json -Compress)"

$r = Call POST "$api/calibrations" @{ panel_device_id = $deviceId; instrument_id = $instId; performed_at = "2026-08-01T09:30:00+07:00"; result = "PASS" }
Show "POST /calibrations (device inactive)" $r

$r = Call POST "$api/panel-devices/$deviceId/restore" $null
Show "POST /panel-devices/(id)/restore" $r

$r = Call DELETE "$api/calibrations/$calId" $null
Show "DELETE /calibrations/(id)" $r
$r = Call GET "$api/calibrations/$calId" $null
Show "GET /calibrations/(id) after delete" $r

Write-Host "`n--- routing ---"
$r = Call GET "$api/nope" $null
Show "GET /api/rtu/v1/nope" $r
$r = Call DELETE "$api/calibrations" $null
Show "DELETE /calibrations (bad method)" $r

