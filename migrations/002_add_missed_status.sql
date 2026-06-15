ALTER TABLE runs
    MODIFY COLUMN status ENUM('running','succeeded','failed','missed') NOT NULL;
