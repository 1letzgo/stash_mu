CREATE TABLE `users` (
  `id` integer not null primary key,
  `username` varchar(255) not null,
  `password_hash` varchar(255) not null default '',
  `role` varchar(16) not null default 'VIEWER',
  `api_key` text,
  `ui_config` text,
  `created_at` datetime not null,
  `updated_at` datetime not null
);

CREATE UNIQUE INDEX `index_users_on_username` ON `users` (`username` COLLATE NOCASE);
CREATE UNIQUE INDEX `index_users_on_api_key` ON `users` (`api_key`) WHERE `api_key` IS NOT NULL AND `api_key` != '';
