BUMP_PART ?=
PYTEST_COV ?= xml
setup:
	python3 -m pip install poetry

install:
	poetry install

test:
	poetry run pytest -ssv --cov --cov-report=$(PYTEST_COV)

build:
	poetry build

publish: build
	poetry publish

format:
	poetry run ruff format moch/ tests/
	poetry run ruff --fix moch/ tests/

lint:
	poetry run ruff moch/ tests/
	poetry run mypy moch/ tests/

bump:
	poetry run bump-my-version bump $(BUMP_PART)
