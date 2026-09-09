-- Equipment specification fields aligned with rtu.calibrations eut_* columns
-- (input_range, accuracy_class, power_supply, output_range).

ALTER TABLE rtu.device_models
    ADD COLUMN input_range    varchar(100),
    ADD COLUMN accuracy_class varchar(100),
    ADD COLUMN power_supply   varchar(100),
    ADD COLUMN output_range   varchar(100);

ALTER TABLE rtu.panel_devices
    ADD COLUMN input_range    varchar(100),
    ADD COLUMN accuracy_class varchar(100),
    ADD COLUMN power_supply   varchar(100),
    ADD COLUMN output_range   varchar(100);
