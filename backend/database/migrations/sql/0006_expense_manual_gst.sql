-- Migration 0006: Add is_manual_gst column to expenses table for manual GST amount overrides
ALTER TABLE "expenses" ADD COLUMN "is_manual_gst" INTEGER DEFAULT 0;
