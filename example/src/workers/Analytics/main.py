import time
import logging

logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(name)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger("AnalyticsWorker")


def process_events():
    logger.info("Waiting for analytics events...")
    time.sleep(2)
    logger.info("Processed a batch of analytics events successfully.")


if __name__ == "__main__":
    logger.info("Starting AnalyticsWorker...")
    while True:
        process_events()
# modified 1773085952
