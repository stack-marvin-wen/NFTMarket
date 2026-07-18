ALTER TABLE ob_collection_sepolia
    ADD COLUMN IF NOT EXISTS banner varchar(1024) NULL COMMENT 'banner video/image uri' AFTER image_uri;

