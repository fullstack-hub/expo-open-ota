package migration

import (
	"expo-open-ota/config"
	"expo-open-ota/internal/bucket"
	"expo-open-ota/internal/cache"
	"fmt"
	"log"
)

func RunMigrations(b bucket.Bucket) error {
	all := All()
	applied, err := b.RetrieveMigrationHistory()
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}
	appliedSet := make(map[string]bool)
	for _, id := range applied {
		appliedSet[id] = true
	}
	for _, m := range all {
		if appliedSet[m.ID()] {
			continue
		}
		fmt.Printf("🔼 Applying migration: %s\n", m.ID())
		if err := m.Up(b); err != nil {
			return fmt.Errorf("migration %s failed: %w", m.ID(), err)
		}
		if err := b.ApplyMigration(m.ID()); err != nil {
			return fmt.Errorf("record migration %s: %w", m.ID(), err)
		}
	}
	return nil
}

func RollbackLastMigration(b bucket.Bucket) error {
	ag, err := b.RetrieveMigrationHistory()
	if err != nil {
		return fmt.Errorf("read history: %w", err)
	}
	if len(ag) == 0 {
		fmt.Println("No migration to rollback.")
		return nil
	}
	last := ag[len(ag)-1]
	var target Migration
	for _, m := range All() {
		if m.ID() == last {
			target = m
			break
		}
	}
	if target == nil {
		return fmt.Errorf("migration %s not found", last)
	}
	fmt.Printf("🔽 Rolling back: %s\n", last)
	if err := target.Down(b); err != nil {
		return fmt.Errorf("rollback %s failed: %w", last, err)
	}
	return b.RemoveMigrationFromHistory(last)
}

func RunMigrationsWithLock() {
	log.Println("🔧 Checking if migrations should run...")
	c := cache.GetCache()

	if config.IsMultiAppMode() {
		for _, app := range config.GetAllApps() {
			appCopy := app
			b := bucket.GetBucketForApp(&appCopy)
			lockKey := fmt.Sprintf("migration-lock:%s", app.Slug)
			ok, err := c.TryLock(lockKey, 120)
			if err != nil {
				log.Fatalf("❌ Failed to acquire migration lock for app %s: %v", app.Slug, err)
			}
			if !ok {
				log.Printf("⏩ Migration already in progress for app %s – skipping.", app.Slug)
				continue
			}
			log.Printf("✅ Migration lock acquired for app %s – starting migrations...", app.Slug)
			if err := RunMigrations(b); err != nil {
				log.Fatalf("🚨 Migration failed for app %s: %v", app.Slug, err)
			}
			log.Printf("🎉 Migrations completed for app %s.", app.Slug)
		}
	} else {
		b := bucket.GetBucket()
		ok, err := c.TryLock("migration-lock", 120)
		if err != nil {
			log.Fatalf("❌ Failed to acquire migration lock: %v", err)
		}
		if !ok {
			log.Println("⏩ Migration already in progress or completed on another instance – skipping.")
			return
		}
		log.Println("✅ Migration lock acquired – starting migrations...")
		if err := RunMigrations(b); err != nil {
			log.Fatalf("🚨 Migration failed: %v", err)
		}
		log.Println("🎉 Migrations completed successfully.")
	}
}
