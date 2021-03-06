/**
* warehouse.c
*
* Group of functions to capture and playback raw data from the
* nfqueue, conntrack, and netlogger handlers for use in testing
* and disgnostics.
*
* Copyright (c) 2018 Untangle, Inc.
* All Rights Reserved
*/

// TODO - add switch to turn off live traffic processing during capture playback

#include "common.h"

static char		*logsrc = "warehouse";
static FILE		*capfile = NULL;

struct data_header {
	char			origin;
	u_int64_t		stamp;
	u_int32_t		length;
	u_int32_t		mark;
	u_int32_t		ctid;
	u_int32_t		nfid;
};

void start_warehouse_capture(void)
{
	logmessage(LOG_INFO,logsrc,"Beginning capture %s\n",get_warehouse_file());
	if (capfile != NULL) fclose(capfile);
	capfile = fopen(get_warehouse_file(),"ab");
}

void close_warehouse_capture(void)
{
	logmessage(LOG_INFO,logsrc,"Finished capture %s\n",get_warehouse_file());
	if (capfile != NULL) fclose(capfile);
	capfile = NULL;
}

void warehouse_capture(const char origin,void *buffer,uint32_t length,uint32_t mark,uint32_t ctid,uint32_t nfid)
{
	struct data_header		dh;
	struct timespec			ts;

	if (get_shutdown_flag() != 0) return;
	if (capfile == NULL) return;

	clock_gettime(CLOCK_MONOTONIC,&ts);
	dh.stamp = (((uint64_t)1000000000 * (uint64_t)ts.tv_sec) + (uint64_t)ts.tv_nsec);
	dh.origin = origin;
	dh.length = length;
	dh.mark = mark;
	dh.ctid = ctid;
	dh.nfid = nfid;
	fwrite(&dh,sizeof(dh),1,capfile);
	fwrite(buffer,length,1,capfile);
}

void warehouse_playback(void)
{
	struct data_header		dh;
	char					*filename;
	char					*buffer;
	FILE					*data;
	size_t					found;
	uint64_t				pause;
	uint64_t				speed;
	uint64_t				last;

	filename = get_warehouse_file();
	data = fopen(filename,"rb");
	if (data == NULL) {
		logmessage(LOG_WARNING,logsrc,"Unable to playback %s\n",filename);
		return;
	}

	speed = get_warehouse_speed();
	last = 0;

	logmessage(LOG_INFO,logsrc,"Beginning playback %s\n",filename);

	for(;;)
	{
		if (feof(data) != 0) break;

		found = fread(&dh,1,sizeof(dh),data);
		if (found != sizeof(dh)) break;

		buffer = malloc(dh.length);
		found = fread(buffer,1,dh.length,data);
		if (found != dh.length) break;

		if (last != 0) pause = (dh.stamp - last);
		else pause = 0;
		last = dh.stamp;

		if ((pause > 0) && (speed > 0))
		{
			pause = (pause / speed);	// apply the speed multiplier
			pause = (pause / 1000);		// convert from nano to micro
			usleep(pause);
		}

		switch(dh.origin)
		{
			case 'Q':
				go_nfqueue_callback(dh.mark,buffer,dh.length,dh.ctid,dh.nfid,buffer);
				break;

			case 'C':
				go_conntrack_callback((struct conntrack_info *)buffer);
				free(buffer);
				break;

			case 'L':
				go_netlogger_callback((struct netlogger_info *)buffer);
				free(buffer);
				break;

			default:
				logmessage(LOG_ERR,logsrc,"Invalid origin packet: %c\n",dh.origin);
		}
	}

	fclose(data);
	logmessage(LOG_INFO,logsrc,"Finished playback %s\n",filename);
}
