"""Forecasting module for FinOpsMind ML sidecar."""

from .prophet import CostForecaster, generate_forecast

__all__ = ['CostForecaster', 'generate_forecast']
