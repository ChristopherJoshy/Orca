-- +goose Up
ALTER TABLE quotations DROP CONSTRAINT IF EXISTS quotations_status_check;
ALTER TABLE quotations ADD CONSTRAINT quotations_status_check CHECK (status IN ('Draft','Sent','Accepted','Converted','Rejected','Expired'));

-- +goose Down
ALTER TABLE quotations DROP CONSTRAINT IF EXISTS quotations_status_check;
ALTER TABLE quotations ADD CONSTRAINT quotations_status_check CHECK (status IN ('Draft','Quotation','Confirmed','Active','Closed'));
