<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onDestroy, onMount } from 'svelte';
	import {
		connectAdmin,
		createChannelMessage,
		createInviteByClient,
		createLiveKitVoiceToken,
		editChannelMessage,
		getChannelMessages,
		getChannels,
		getHealth,
		getServerInfo,
		getVoiceChannelState,
		leaveVoiceChannel,
		listInvitesByClient,
		openChannelStream,
		touchVoicePresence,
		type ChannelMessage,
		type ChannelStreamEvent,
		type InviteSummary,
		type VoiceParticipant
	} from '$lib/api';
	import {
		createAdminInviteSignature,
		createAdminListInvitesSignature,
		createAdminSessionSignature
	} from '$lib/crypto';
	import { renderMarkdown } from '$lib/markdown';
	import {
		getServerByID,
		loadIdentity,
		removeServerByID,
		resetLocalState,
		upsertServer
	} from '$lib/storage';
	import type { Channel, IdentityRecord, SavedServer } from '$lib/types';
	import { Room, RoomEvent, Track, type RemoteTrack, type RemoteTrackPublication } from 'livekit-client';

	let identity: IdentityRecord | null = null;
	let server: SavedServer | null = null;
	let channels: Channel[] = [];
	let backendStatus: 'ok' | 'fail' = 'fail';
	let loading = true;
	let error = '';
	let adminPublicKeys: string[] = [];

	let targetClientPublicKey = '';
	let targetClientLabel = '';
	let creatingInvite = false;
	let createInviteError = '';
	let createdInviteLink = '';

	let loadingInvites = false;
	let invitesError = '';
	let invitesLoaded = false;
	let invites: InviteSummary[] = [];

	let textMessages: ChannelMessage[] = [];
	let loadingTextMessages = false;
	let textMessagesError = '';
	let messageDraft = '';
	let sendingMessage = false;
	let editingMessageID = '';
	let editingDraft = '';

	let streamSocket: WebSocket | null = null;
	let streamChannelID = '';

	let voiceRoom: Room | null = null;
	let activeVoiceChannelID = '';
	let voiceParticipants: VoiceParticipant[] = [];
	let voiceConnecting = false;
	let voiceLoadingState = false;
	let voiceError = '';
	let localMicEnabled = false;
	let localCameraEnabled = false;
	let localScreenEnabled = false;
	let localScreenAudioEnabled = false;
	let screenShareWithAudio = false;
	let remoteVideoSources: Array<{
		trackSid: string;
		ownerPublicKey: string;
		ownerName: string;
		source: 'camera' | 'screen';
	}> = [];
	let watchedVideoTrackSIDs: string[] = [];
	let mediaSinkElement: HTMLDivElement | null = null;
	let videoGridElement: HTMLDivElement | null = null;
	let voiceStateTimer: ReturnType<typeof setInterval> | null = null;
	let voicePresenceTimer: ReturnType<typeof setInterval> | null = null;
	const attachedAudioElements = new Map<string, HTMLMediaElement>();
	const attachedVideoElements = new Map<string, HTMLMediaElement>();

	let initialized = false;
	let activeServerID = '';
	let autoAdminLoginInProgress = false;

	$: currentView = $page.url.searchParams.get('view') ?? 'channel';
	$: currentChannelID = $page.url.searchParams.get('channel') ?? '';
	$: selectedChannel =
		channels.find((channel) => channel.id === currentChannelID) ?? channels[0] ?? null;
	$: isAdmin = Boolean(identity && adminPublicKeys.includes(identity.publicKey));
	$: selectedTextChannelID =
		currentView !== 'admin' && selectedChannel?.type === 'text' ? selectedChannel.id : '';
	$: selectedVoiceChannelID =
		currentView !== 'admin' && selectedChannel?.type === 'voice' ? selectedChannel.id : '';

	onMount(() => {
		initialized = true;
	});

	onDestroy(() => {
		closeStream();
		void disconnectVoiceChannel();
	});

	function closeStream() {
		if (streamSocket) {
			streamSocket.close();
			streamSocket = null;
		}
	}

	async function handleForgetServer() {
		if (!server) {
			return;
		}

		if (
			typeof window !== 'undefined' &&
			!window.confirm(`Forget server "${server.name}" and remove local connection state?`)
		) {
			return;
		}

		await disconnectVoiceChannel();
		closeStream();
		removeServerByID(server.id);

		server = null;
		channels = [];
		adminPublicKeys = [];
		textMessages = [];
		voiceParticipants = [];

		await goto('/');
	}

	async function handleResetLocalState() {
		if (
			typeof window !== 'undefined' &&
			!window.confirm(
				'Reset local state? This will remove your client identity and all saved servers from this device.'
			)
		) {
			return;
		}

		await disconnectVoiceChannel();
		closeStream();
		resetLocalState();

		identity = null;
		server = null;
		channels = [];
		adminPublicKeys = [];
		textMessages = [];
		voiceParticipants = [];

		await goto('/');
	}

	function shortKey(value: string): string {
		if (value.length <= 20) {
			return value;
		}
		return `${value.slice(0, 10)}...${value.slice(-8)}`;
	}

	function formatTimestamp(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) {
			return value;
		}
		return date.toLocaleString();
	}

	function sortMessages(messages: ChannelMessage[]): ChannelMessage[] {
		return [...messages].sort((a, b) => a.createdAt.localeCompare(b.createdAt));
	}

	function upsertMessage(message: ChannelMessage) {
		const index = textMessages.findIndex((item) => item.id === message.id);
		if (index >= 0) {
			textMessages[index] = message;
		} else {
			textMessages = [...textMessages, message];
		}
		textMessages = sortMessages(textMessages);
	}

	function connectTextStream(channelID: string) {
		if (!server?.sessionToken) {
			return;
		}

		closeStream();

		const socket = openChannelStream({
			channelId: channelID,
			sessionToken: server.sessionToken,
			baseUrl: server.baseUrl
		});

		socket.onmessage = (event) => {
			try {
				const parsed = JSON.parse(event.data) as ChannelStreamEvent;
				if ((parsed.type === 'message.created' || parsed.type === 'message.updated') && parsed.message) {
					upsertMessage(parsed.message);
				}
			} catch {
				// Ignore malformed websocket messages.
			}
		};

		socket.onerror = () => {
			textMessagesError = 'Live updates connection failed.';
		};

		streamSocket = socket;
	}

	function toLiveKitWebSocketURL(rawURL: string): string {
		const value = rawURL.trim();
		if (!value) {
			return '';
		}
		if (value.startsWith('ws://') || value.startsWith('wss://')) {
			return value;
		}
		if (value.startsWith('http://')) {
			return `ws://${value.slice('http://'.length)}`;
		}
		if (value.startsWith('https://')) {
			return `wss://${value.slice('https://'.length)}`;
		}
		return value;
	}

	function stopVoiceTimers() {
		if (voiceStateTimer) {
			clearInterval(voiceStateTimer);
			voiceStateTimer = null;
		}
		if (voicePresenceTimer) {
			clearInterval(voicePresenceTimer);
			voicePresenceTimer = null;
		}
	}

	function clearMediaElements() {
		for (const element of attachedAudioElements.values()) {
			element.remove();
		}
		attachedAudioElements.clear();

		for (const element of attachedVideoElements.values()) {
			element.remove();
		}
		attachedVideoElements.clear();
	}

	function collectLocalVoicePresence() {
		let audioStreams = 0;
		let videoStreams = 0;
		let cameraEnabled = false;
		let screenEnabled = false;
		let screenAudioEnabled = false;

		const room = voiceRoom;
		if (room) {
			for (const publication of room.localParticipant.trackPublications.values()) {
				if (!publication.track) {
					continue;
				}
				if (publication.kind === Track.Kind.Audio) {
					audioStreams += 1;
				}
				if (publication.kind === Track.Kind.Video) {
					videoStreams += 1;
				}
				if (publication.source === Track.Source.Camera) {
					cameraEnabled = true;
				}
				if (publication.source === Track.Source.ScreenShare) {
					screenEnabled = true;
				}
				if (publication.source === Track.Source.ScreenShareAudio) {
					screenAudioEnabled = true;
				}
			}
		}

		return {
			audioStreams,
			videoStreams,
			cameraEnabled,
			screenEnabled,
			screenAudioEnabled
		};
	}

	function syncLocalTrackFlags() {
		const summary = collectLocalVoicePresence();
		localMicEnabled = summary.audioStreams > 0;
		localCameraEnabled = summary.cameraEnabled;
		localScreenEnabled = summary.screenEnabled;
		localScreenAudioEnabled = summary.screenAudioEnabled;
	}

	async function refreshVoiceState(channelID: string) {
		if (!server?.sessionToken) {
			return;
		}
		if (activeVoiceChannelID !== channelID) {
			return;
		}

		voiceLoadingState = true;
		try {
			const response = await getVoiceChannelState({
				channelId: channelID,
				sessionToken: server.sessionToken,
				baseUrl: server.baseUrl
			});
			if (activeVoiceChannelID === channelID) {
				voiceParticipants = response.participants;
			}
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to load voice channel state';
		} finally {
			voiceLoadingState = false;
		}
	}

	async function pushVoicePresence(channelID: string) {
		if (!server?.sessionToken || !voiceRoom || activeVoiceChannelID !== channelID) {
			return;
		}
		const summary = collectLocalVoicePresence();

		try {
			await touchVoicePresence({
				channelId: channelID,
				sessionToken: server.sessionToken,
				baseUrl: server.baseUrl,
				audioStreams: summary.audioStreams,
				videoStreams: summary.videoStreams,
				cameraEnabled: summary.cameraEnabled,
				screenEnabled: summary.screenEnabled,
				screenAudioEnabled: summary.screenAudioEnabled
			});
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to update voice state';
		}
	}

	function startVoiceTimers(channelID: string) {
		stopVoiceTimers();
		voiceStateTimer = setInterval(() => {
			void refreshVoiceState(channelID);
		}, 3000);
		voicePresenceTimer = setInterval(() => {
			void pushVoicePresence(channelID);
		}, 5000);
	}

	function attachRemoteAudio(track: RemoteTrack, trackSid: string) {
		if (!mediaSinkElement || track.kind !== Track.Kind.Audio) {
			return;
		}

		let element = attachedAudioElements.get(trackSid);
		if (!element) {
			element = track.attach();
			element.autoplay = true;
			element.setAttribute('playsinline', 'true');
			element.classList.add('hidden-media');
			attachedAudioElements.set(trackSid, element);
			mediaSinkElement.appendChild(element);
			return;
		}

		track.attach(element);
	}

	function detachRemoteAudio(trackSid: string) {
		const existing = attachedAudioElements.get(trackSid);
		if (!existing) {
			return;
		}
		existing.remove();
		attachedAudioElements.delete(trackSid);
	}

	function findRemoteVideoTrack(trackSid: string): RemoteTrack | null {
		if (!voiceRoom) {
			return null;
		}
		for (const participant of voiceRoom.remoteParticipants.values()) {
			for (const publication of participant.trackPublications.values()) {
				if (publication.trackSid !== trackSid) {
					continue;
				}
				if (!publication.track || publication.kind !== Track.Kind.Video) {
					continue;
				}
				return publication.track;
			}
		}
		return null;
	}

	function attachWatchedVideo(trackSid: string) {
		if (!videoGridElement) {
			return;
		}
		if (attachedVideoElements.has(trackSid)) {
			return;
		}

		const track = findRemoteVideoTrack(trackSid);
		if (!track || track.kind !== Track.Kind.Video) {
			return;
		}

		const element = track.attach();
		element.autoplay = true;
		element.setAttribute('playsinline', 'true');
		element.classList.add('remote-video');
		attachedVideoElements.set(trackSid, element);
		videoGridElement.appendChild(element);
	}

	function detachWatchedVideo(trackSid: string) {
		const existing = attachedVideoElements.get(trackSid);
		if (!existing) {
			return;
		}
		existing.remove();
		attachedVideoElements.delete(trackSid);
	}

	function isWatchingVideo(trackSid: string): boolean {
		return watchedVideoTrackSIDs.includes(trackSid);
	}

	function watchVideoTrack(trackSid: string) {
		if (!isWatchingVideo(trackSid)) {
			watchedVideoTrackSIDs = [...watchedVideoTrackSIDs, trackSid];
		}
		attachWatchedVideo(trackSid);
	}

	function unwatchVideoTrack(trackSid: string) {
		watchedVideoTrackSIDs = watchedVideoTrackSIDs.filter((item) => item !== trackSid);
		detachWatchedVideo(trackSid);
	}

	function toggleWatchVideo(trackSid: string) {
		if (isWatchingVideo(trackSid)) {
			unwatchVideoTrack(trackSid);
			return;
		}
		watchVideoTrack(trackSid);
	}

	function updateRemoteVideoSources() {
		if (!voiceRoom) {
			remoteVideoSources = [];
			watchedVideoTrackSIDs = [];
			clearMediaElements();
			return;
		}

		const collected: Array<{
			trackSid: string;
			ownerPublicKey: string;
			ownerName: string;
			source: 'camera' | 'screen';
		}> = [];

		for (const participant of voiceRoom.remoteParticipants.values()) {
			for (const publication of participant.trackPublications.values()) {
				if (publication.kind !== Track.Kind.Video) {
					continue;
				}
				if (
					publication.source !== Track.Source.Camera &&
					publication.source !== Track.Source.ScreenShare
				) {
					continue;
				}
				const trackSid = publication.trackSid;
				if (!trackSid) {
					continue;
				}
				collected.push({
					trackSid,
					ownerPublicKey: participant.identity,
					ownerName: participant.name?.trim() || participant.identity,
					source: publication.source === Track.Source.Camera ? 'camera' : 'screen'
				});
			}
		}

		remoteVideoSources = collected;
		const available = new Set(collected.map((item) => item.trackSid));
		for (const watched of [...watchedVideoTrackSIDs]) {
			if (!available.has(watched)) {
				unwatchVideoTrack(watched);
			}
		}
	}

	async function disconnectVoiceChannel() {
		stopVoiceTimers();

		const previousServer = server;
		const previousChannel = activeVoiceChannelID;
		const currentRoom = voiceRoom;
		voiceRoom = null;
		activeVoiceChannelID = '';

		if (currentRoom) {
			currentRoom.removeAllListeners();
			await currentRoom.disconnect(true);
		}

		clearMediaElements();
		remoteVideoSources = [];
		watchedVideoTrackSIDs = [];
		localMicEnabled = false;
		localCameraEnabled = false;
		localScreenEnabled = false;
		localScreenAudioEnabled = false;
		voiceParticipants = [];

		if (previousServer?.sessionToken && previousChannel) {
			try {
				await leaveVoiceChannel({
					sessionToken: previousServer.sessionToken,
					baseUrl: previousServer.baseUrl
				});
			} catch {
				// Best effort cleanup on server-side presence.
			}
		}
	}

	function registerRoomHandlers(room: Room, channelID: string) {
		room.on(RoomEvent.TrackSubscribed, (track, publication) => {
			if (track.kind === Track.Kind.Audio) {
				attachRemoteAudio(track, publication.trackSid);
			}
			if (track.kind === Track.Kind.Video && isWatchingVideo(publication.trackSid)) {
				attachWatchedVideo(publication.trackSid);
			}
			updateRemoteVideoSources();
		});

		room.on(RoomEvent.TrackUnsubscribed, (_track, publication) => {
			detachRemoteAudio(publication.trackSid);
			detachWatchedVideo(publication.trackSid);
			updateRemoteVideoSources();
		});

		room.on(RoomEvent.TrackPublished, () => {
			updateRemoteVideoSources();
		});
		room.on(RoomEvent.TrackUnpublished, (publication: RemoteTrackPublication) => {
			detachRemoteAudio(publication.trackSid);
			detachWatchedVideo(publication.trackSid);
			updateRemoteVideoSources();
		});
		room.on(RoomEvent.ParticipantConnected, () => {
			updateRemoteVideoSources();
			void refreshVoiceState(channelID);
		});
		room.on(RoomEvent.ParticipantDisconnected, () => {
			updateRemoteVideoSources();
			void refreshVoiceState(channelID);
		});
		room.on(RoomEvent.LocalTrackPublished, () => {
			syncLocalTrackFlags();
			void pushVoicePresence(channelID);
		});
		room.on(RoomEvent.LocalTrackUnpublished, () => {
			syncLocalTrackFlags();
			void pushVoicePresence(channelID);
		});
	}

	async function joinVoiceChannel(channelID: string) {
		if (!server?.sessionToken) {
			voiceError = 'Missing session token. Reconnect using an invite link.';
			return;
		}

		voiceConnecting = true;
		voiceError = '';
		await disconnectVoiceChannel();

		const activeServer = server;
		if (!activeServer?.sessionToken) {
			voiceConnecting = false;
			voiceError = 'Missing session token. Reconnect using an invite link.';
			return;
		}

		try {
			const tokenResponse = await createLiveKitVoiceToken({
				channelId: channelID,
				sessionToken: activeServer.sessionToken,
				baseUrl: activeServer.baseUrl
			});

			const liveKitURL = toLiveKitWebSocketURL(activeServer.livekitUrl);
			if (!liveKitURL) {
				throw new Error('LiveKit URL is not configured');
			}

			const room = new Room();
			registerRoomHandlers(room, channelID);

			await room.connect(liveKitURL, tokenResponse.token);
			await room.localParticipant.setMicrophoneEnabled(true);

			voiceRoom = room;
			activeVoiceChannelID = channelID;
			syncLocalTrackFlags();
			updateRemoteVideoSources();
			await pushVoicePresence(channelID);
			await refreshVoiceState(channelID);
			startVoiceTimers(channelID);
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to join voice channel';
			try {
				await leaveVoiceChannel({
					sessionToken: activeServer.sessionToken,
					baseUrl: activeServer.baseUrl
				});
			} catch {
				// Ignore cleanup failure and continue local disconnect.
			}
			await disconnectVoiceChannel();
		} finally {
			voiceConnecting = false;
		}
	}

	async function toggleMicrophone() {
		if (!voiceRoom || !activeVoiceChannelID) {
			return;
		}
		try {
			await voiceRoom.localParticipant.setMicrophoneEnabled(!localMicEnabled);
			syncLocalTrackFlags();
			await pushVoicePresence(activeVoiceChannelID);
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to toggle microphone';
		}
	}

	async function toggleCamera() {
		if (!voiceRoom || !activeVoiceChannelID) {
			return;
		}
		try {
			await voiceRoom.localParticipant.setCameraEnabled(!localCameraEnabled);
			syncLocalTrackFlags();
			await pushVoicePresence(activeVoiceChannelID);
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to toggle camera';
		}
	}

	async function toggleScreenShare() {
		if (!voiceRoom || !activeVoiceChannelID) {
			return;
		}
		try {
			await voiceRoom.localParticipant.setScreenShareEnabled(!localScreenEnabled, {
				audio: screenShareWithAudio
			});
			syncLocalTrackFlags();
			await pushVoicePresence(activeVoiceChannelID);
		} catch (e) {
			voiceError = e instanceof Error ? e.message : 'Failed to toggle screen sharing';
		}
	}

	async function loadTextMessages(channelID: string) {
		if (!server?.sessionToken || !server) {
			textMessagesError = 'Missing session token. Reconnect using an invite link.';
			textMessages = [];
			return;
		}

		loadingTextMessages = true;
		textMessagesError = '';
		editingMessageID = '';
		editingDraft = '';

		try {
			const response = await getChannelMessages({
				channelId: channelID,
				sessionToken: server.sessionToken,
				baseUrl: server.baseUrl,
				limit: 100
			});
			textMessages = sortMessages(response.messages);
			connectTextStream(channelID);
		} catch (e) {
			textMessagesError = e instanceof Error ? e.message : 'Failed to load messages';
			textMessages = [];
			closeStream();
		} finally {
			loadingTextMessages = false;
		}
	}

	async function initializeServer(serverID: string) {
		activeServerID = serverID;
		await disconnectVoiceChannel();
		error = '';
		loading = true;
		invitesLoaded = false;
		invites = [];
		invitesError = '';
		adminPublicKeys = [];
		textMessages = [];
		textMessagesError = '';
		streamChannelID = '';
		closeStream();

		if (!serverID) {
			error = 'Missing server id';
			loading = false;
			return;
		}

		identity = loadIdentity();
		if (!identity) {
			void goto('/setup');
			return;
		}

		server = getServerByID(serverID);
		if (!server) {
			error = `Unknown server id: ${serverID}`;
			loading = false;
			return;
		}

		await refreshServerState();
	}

	async function refreshServerState() {
		if (!server) {
			return;
		}

		try {
			const [health, channelResponse, serverInfo] = await Promise.all([
				getHealth(server.baseUrl),
				getChannels(server.baseUrl),
				getServerInfo(server.baseUrl)
			]);

			backendStatus = health.status;
			channels = channelResponse.channels;
			adminPublicKeys = serverInfo.adminPublicKeys ?? [];

			server = {
				...server,
				name: serverInfo.name,
				serverFingerprint: serverInfo.serverFingerprint,
				livekitUrl: serverInfo.livekitUrl,
				sessionToken: server.sessionToken,
				channels,
				lastConnectedAt: new Date().toISOString()
			};

			if (!server.sessionToken) {
				await tryAutoAdminLogin(serverInfo.serverFingerprint);
			}

			upsertServer(server);

			if (currentView !== 'admin' && !currentChannelID && channels.length > 0) {
				await goto(`/server/${server.id}?view=channel&channel=${encodeURIComponent(channels[0].id)}`, {
					replaceState: true,
					noScroll: true
				});
			}
		} catch (e) {
			error = e instanceof Error ? e.message : 'Unknown error';
		} finally {
			loading = false;
		}
	}

	async function tryAutoAdminLogin(serverFingerprint: string) {
		if (!server || !identity || autoAdminLoginInProgress) {
			return;
		}
		if (server.sessionToken) {
			return;
		}
		if (!adminPublicKeys.includes(identity.publicKey)) {
			return;
		}

		autoAdminLoginInProgress = true;
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminSessionSignature({
				adminPublicKey: identity.publicKey,
				issuedAt,
				serverFingerprint,
				adminPrivateKeyBase64: identity.privateKey
			});

			const result = await connectAdmin(
				{
					adminPublicKey: identity.publicKey,
					issuedAt,
					signature,
					clientInfo: {
						displayName: 'Admin Client'
					}
				},
				server.baseUrl
			);

			channels = result.channels;
			server = {
				...server,
				id: result.serverId,
				name: result.serverName,
				serverFingerprint: result.serverFingerprint,
				livekitUrl: result.livekitUrl,
				sessionToken: result.sessionToken,
				channels: result.channels,
				lastConnectedAt: new Date().toISOString()
			};
			upsertServer(server);
		} catch {
			// Keep normal pre-login behavior when admin auto-login fails.
		} finally {
			autoAdminLoginInProgress = false;
		}
	}

	async function openAdmin() {
		if (!server) {
			return;
		}
		await goto(`/server/${server.id}?view=admin`, { noScroll: true });
	}

	async function openChannel(channelID: string) {
		if (!server) {
			return;
		}
		await goto(`/server/${server.id}?view=channel&channel=${encodeURIComponent(channelID)}`, {
			noScroll: true
		});
	}

	async function refreshInvites() {
		if (!server || !identity || !isAdmin) {
			return;
		}

		loadingInvites = true;
		invitesError = '';
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminListInvitesSignature({
				adminPublicKey: identity.publicKey,
				issuedAt,
				adminPrivateKeyBase64: identity.privateKey
			});

			const response = await listInvitesByClient(
				{
					adminPublicKey: identity.publicKey,
					issuedAt,
					signature
				},
				server.baseUrl
			);
			invites = response.invites;
		} catch (e) {
			invitesError = e instanceof Error ? e.message : 'Failed to load invites';
		} finally {
			invitesLoaded = true;
			loadingInvites = false;
		}
	}

	async function handleCreateInvite() {
		if (!server || !identity) {
			createInviteError = 'Identity is not available';
			return;
		}
		if (!targetClientPublicKey.trim()) {
			createInviteError = 'Target client public key is required';
			return;
		}

		creatingInvite = true;
		createInviteError = '';
		createdInviteLink = '';
		try {
			const issuedAt = new Date().toISOString();
			const signature = await createAdminInviteSignature({
				adminPublicKey: identity.publicKey,
				clientPublicKey: targetClientPublicKey.trim(),
				issuedAt,
				adminPrivateKeyBase64: identity.privateKey
			});

			const result = await createInviteByClient(
				{
					adminPublicKey: identity.publicKey,
					clientPublicKey: targetClientPublicKey.trim(),
					label: targetClientLabel.trim(),
					issuedAt,
					signature
				},
				server.baseUrl
			);
			createdInviteLink = result.inviteLink;
			targetClientPublicKey = '';
			targetClientLabel = '';
			await refreshInvites();
		} catch (e) {
			createInviteError = e instanceof Error ? e.message : 'Failed to create invite';
		} finally {
			creatingInvite = false;
		}
	}

	async function handleSendMessage() {
		if (!server?.sessionToken || !selectedTextChannelID || !messageDraft.trim()) {
			return;
		}

		sendingMessage = true;
		textMessagesError = '';
		try {
			const response = await createChannelMessage({
				channelId: selectedTextChannelID,
				sessionToken: server.sessionToken,
				contentMarkdown: messageDraft.trim(),
				baseUrl: server.baseUrl
			});
			upsertMessage(response.message);
			messageDraft = '';
		} catch (e) {
			textMessagesError = e instanceof Error ? e.message : 'Failed to send message';
		} finally {
			sendingMessage = false;
		}
	}

	function startEditMessage(message: ChannelMessage) {
		editingMessageID = message.id;
		editingDraft = message.contentMarkdown;
	}

	async function saveEditMessage() {
		if (!server?.sessionToken || !selectedTextChannelID || !editingMessageID || !editingDraft.trim()) {
			return;
		}

		textMessagesError = '';
		try {
			const response = await editChannelMessage({
				channelId: selectedTextChannelID,
				messageId: editingMessageID,
				sessionToken: server.sessionToken,
				contentMarkdown: editingDraft.trim(),
				baseUrl: server.baseUrl
			});
			upsertMessage(response.message);
			editingMessageID = '';
			editingDraft = '';
		} catch (e) {
			textMessagesError = e instanceof Error ? e.message : 'Failed to edit message';
		}
	}

	$: if (currentView === 'admin' && isAdmin && !loading && !invitesLoaded && !loadingInvites) {
		void refreshInvites();
	}

	$: if (initialized) {
		const serverID = $page.params.id;
		if (serverID && serverID !== activeServerID) {
			void initializeServer(serverID);
		}
	}

	$: if (selectedTextChannelID && selectedTextChannelID !== streamChannelID && !loading) {
		streamChannelID = selectedTextChannelID;
		void loadTextMessages(selectedTextChannelID);
	}

	$: if (!selectedTextChannelID) {
		streamChannelID = '';
		textMessages = [];
		textMessagesError = '';
		closeStream();
	}

	$: if (selectedVoiceChannelID && selectedVoiceChannelID !== activeVoiceChannelID && !loading) {
		void joinVoiceChannel(selectedVoiceChannelID);
	}

	$: if (!selectedVoiceChannelID && activeVoiceChannelID && !loading) {
		void disconnectVoiceChannel();
	}
</script>

{#if server}
	<div class="server-layout">
		<aside class="channel-sidebar">
			<header class="server-header">
				<h1>{server.name}</h1>
				<p>{server.serverFingerprint}</p>
			</header>

			<section class="sidebar-group">
				<h2>Channels</h2>
				{#if channels.length === 0}
					<p class="muted">No channels available</p>
				{:else}
					{#each channels as channel}
						<button
							class="nav-item"
							class:active={currentView !== 'admin' && selectedChannel?.id === channel.id}
							on:click={() => openChannel(channel.id)}
						>
							<span class="icon">{channel.type === 'voice' ? '🔊' : '#'}</span>
							<span>{channel.name}</span>
						</button>
					{/each}
				{/if}
			</section>

			<section class="sidebar-group">
				<h2>Pages</h2>
				<button class="nav-item" class:active={currentView === 'admin'} on:click={openAdmin}>
					<span class="icon">🛠</span>
					<span>Admin</span>
				</button>
			</section>
		</aside>

		<section class="server-content">
			{#if loading}
				<p>Loading server data...</p>
			{:else if error}
				<p class="error">{error}</p>
			{:else if currentView === 'admin'}
				<h2>Admin</h2>
				{#if !isAdmin}
					<p class="error">You are not a server administrator.</p>
				{:else}
					<section class="card">
						<h3>Create Invite</h3>
						<label for="target-key">Client public key</label>
						<textarea
							id="target-key"
							bind:value={targetClientPublicKey}
							rows="4"
							placeholder="Base64 Ed25519 public key"
						></textarea>
						<label for="target-label">Label (optional)</label>
						<input id="target-label" bind:value={targetClientLabel} placeholder="Optional label" />
						<div class="actions-row">
							<button on:click={handleCreateInvite} disabled={creatingInvite || !targetClientPublicKey.trim()}>
								{creatingInvite ? 'Creating...' : 'Create Invite'}
							</button>
							<button class="ghost" on:click={refreshInvites} disabled={loadingInvites}>Refresh</button>
						</div>
						{#if createInviteError}
							<p class="error">{createInviteError}</p>
						{/if}
						{#if createdInviteLink}
							<p>Invite link:</p>
							<code class="block">{createdInviteLink}</code>
						{/if}
					</section>

					<section class="card">
						<h3>Invites</h3>
						{#if loadingInvites}
							<p>Loading invites...</p>
						{:else if invitesError}
							<p class="error">{invitesError}</p>
						{:else if invites.length === 0}
							<p>No invites yet.</p>
						{:else}
							<table>
								<thead>
									<tr>
										<th>Status</th>
										<th>Invite ID</th>
										<th>Label</th>
										<th>Client Key</th>
										<th>Created</th>
									</tr>
								</thead>
								<tbody>
									{#each invites as invite}
										<tr>
											<td>
												<span class:used={invite.status === 'used'}>{invite.status}</span>
											</td>
											<td><code>{invite.inviteId}</code></td>
											<td>{invite.label || '-'}</td>
											<td><code>{shortKey(invite.allowedClientPublicKey)}</code></td>
											<td>{formatTimestamp(invite.createdAt)}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</section>
				{/if}
			{:else if selectedChannel?.type === 'text'}
				<h2>{selectedChannel.name}</h2>
				<p class="muted">
					Backend: <strong class={backendStatus === 'ok' ? 'ok' : 'fail'}>{backendStatus}</strong>
				</p>
				{#if !server.sessionToken}
					<p class="error">Missing session token. Reconnect using an invite link.</p>
					<div class="actions-row">
						<button class="ghost" on:click={handleForgetServer}>Leave server</button>
						<button class="danger" on:click={handleResetLocalState}>Reset local state</button>
					</div>
				{:else}
					<section class="chat-shell">
						<div class="message-list">
							{#if loadingTextMessages}
								<p>Loading recent messages...</p>
							{:else if textMessagesError}
								<p class="error">{textMessagesError}</p>
							{:else if textMessages.length === 0}
								<p class="muted">No messages yet.</p>
							{:else}
								{#each textMessages as message}
									<article class="message-item">
										<header>
											<strong>{message.author.displayName}</strong>
											<code>{shortKey(message.author.publicKey)}</code>
											<span>{formatTimestamp(message.createdAt)}</span>
											{#if message.updatedAt !== message.createdAt}
												<em>(edited)</em>
											{/if}
											<button class="link-btn" on:click={() => startEditMessage(message)}>Edit</button>
										</header>
										{#if editingMessageID === message.id}
											<div class="edit-box">
												<textarea bind:value={editingDraft} rows="3"></textarea>
												<div class="actions-row">
													<button on:click={saveEditMessage} disabled={!editingDraft.trim()}>Save</button>
													<button
														class="ghost"
														on:click={() => {
															editingMessageID = '';
															editingDraft = '';
														}}
													>
														Cancel
													</button>
												</div>
											</div>
										{:else}
											<div class="markdown-body">{@html renderMarkdown(message.contentMarkdown)}</div>
										{/if}
									</article>
								{/each}
							{/if}
						</div>

						<div class="composer">
							<textarea
								bind:value={messageDraft}
								rows="4"
								placeholder="Write a message in Markdown..."
							></textarea>
							<div class="actions-row">
								<button on:click={handleSendMessage} disabled={sendingMessage || !messageDraft.trim()}>
									{sendingMessage ? 'Sending...' : 'Send'}
								</button>
							</div>
						</div>
					</section>
				{/if}
			{:else}
				<h2>{selectedChannel ? selectedChannel.name : 'Channel'}</h2>
				<p class="muted">
					Voice status:
					<strong class={activeVoiceChannelID ? 'ok' : 'fail'}>
						{activeVoiceChannelID ? 'connected' : 'disconnected'}
					</strong>
				</p>

				{#if !server.sessionToken}
					<p class="error">Missing session token. Reconnect using an invite link.</p>
					<div class="actions-row">
						<button class="ghost" on:click={handleForgetServer}>Leave server</button>
						<button class="danger" on:click={handleResetLocalState}>Reset local state</button>
					</div>
				{:else}
					<section class="card">
						<h3>Voice Controls</h3>
						<div class="actions-row">
							<button on:click={toggleMicrophone} disabled={voiceConnecting || !activeVoiceChannelID}>
								{localMicEnabled ? 'Mute mic' : 'Unmute mic'}
							</button>
							<button on:click={toggleCamera} disabled={voiceConnecting || !activeVoiceChannelID}>
								{localCameraEnabled ? 'Stop camera' : 'Start camera'}
							</button>
							<button on:click={toggleScreenShare} disabled={voiceConnecting || !activeVoiceChannelID}>
								{localScreenEnabled ? 'Stop share' : 'Share screen'}
							</button>
						</div>
						<label class="checkbox-row">
							<input type="checkbox" bind:checked={screenShareWithAudio} />
							<span>Include system audio when sharing screen</span>
						</label>
						<p class="muted">LiveKit URL: <code>{server.livekitUrl}</code></p>
						{#if voiceConnecting}
							<p>Connecting to voice channel...</p>
						{/if}
						{#if voiceError}
							<p class="error">{voiceError}</p>
						{/if}
					</section>

					<section class="card">
						<h3>Users In Channel</h3>
						{#if voiceLoadingState}
							<p>Loading voice state...</p>
						{:else if voiceParticipants.length === 0}
							<p class="muted">No users in this voice channel yet.</p>
						{:else}
							<table>
								<thead>
									<tr>
										<th>User</th>
										<th>Public Key</th>
										<th>Audio Streams</th>
										<th>Video Streams</th>
										<th>Camera</th>
										<th>Screen</th>
									</tr>
								</thead>
								<tbody>
									{#each voiceParticipants as participant}
										<tr>
											<td>{participant.displayName}</td>
											<td><code>{shortKey(participant.publicKey)}</code></td>
											<td>{participant.audioStreams}</td>
											<td>{participant.videoStreams}</td>
											<td>{participant.cameraEnabled ? 'on' : 'off'}</td>
											<td>{participant.screenEnabled ? 'on' : 'off'}</td>
										</tr>
									{/each}
								</tbody>
							</table>
						{/if}
					</section>

					<section class="card">
						<h3>Video Streams</h3>
						{#if remoteVideoSources.length === 0}
							<p class="muted">No remote camera/screen streams available.</p>
						{:else}
							<div class="video-source-list">
								{#each remoteVideoSources as source}
									<div class="video-source-item">
										<div>
											<strong>{source.source === 'camera' ? 'Camera' : 'Screen'}</strong>
											<span>{source.ownerName}</span>
											<code>{shortKey(source.ownerPublicKey)}</code>
										</div>
										<button on:click={() => toggleWatchVideo(source.trackSid)}>
											{isWatchingVideo(source.trackSid) ? 'Stop watching' : 'Watch'}
										</button>
									</div>
								{/each}
							</div>
						{/if}

						<div class="video-grid" bind:this={videoGridElement}></div>
						<div class="hidden-media-sink" bind:this={mediaSinkElement}></div>
					</section>
				{/if}
			{/if}
		</section>
	</div>
{/if}

<style>
	.server-layout {
		display: grid;
		grid-template-columns: 280px minmax(0, 1fr);
		gap: 0;
		background: #111724;
		border: 1px solid #273149;
		border-radius: 12px;
		min-height: calc(100vh - 120px);
		overflow: hidden;
	}

	.channel-sidebar {
		background: #1b2233;
		border-right: 1px solid #273149;
		padding: 12px;
		display: flex;
		flex-direction: column;
		gap: 14px;
	}

	.server-header h1 {
		margin: 0;
		font-size: 18px;
	}

	.server-header p {
		margin: 4px 0 0;
		color: #9fb1cf;
	}

	.sidebar-group h2 {
		margin: 0 0 8px;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: #8ea5c9;
	}

	.nav-item {
		width: 100%;
		text-align: left;
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 10px;
		border-radius: 8px;
		border: 1px solid transparent;
		background: transparent;
		color: #d6deee;
		cursor: pointer;
		margin-bottom: 4px;
	}

	.nav-item:hover {
		background: #25304a;
	}

	.nav-item.active {
		background: #2f63ff;
		border-color: #83a3ff;
		color: #ffffff;
	}

	.icon {
		width: 20px;
		text-align: center;
	}

	.server-content {
		padding: 16px;
		overflow: auto;
	}

	.card {
		margin-top: 12px;
		padding: 14px;
		border-radius: 10px;
		border: 1px solid #2f3c58;
		background: #151c2b;
	}

	.chat-shell {
		margin-top: 12px;
		display: grid;
		grid-template-rows: minmax(0, 1fr) auto;
		gap: 12px;
		min-height: 560px;
	}

	.message-list {
		border: 1px solid #2f3c58;
		border-radius: 10px;
		padding: 12px;
		background: #151c2b;
		overflow: auto;
	}

	.message-item {
		border-bottom: 1px solid #2f3c58;
		padding: 10px 0;
	}

	.message-item:last-child {
		border-bottom: 0;
	}

	.message-item header {
		display: flex;
		align-items: center;
		gap: 8px;
		font-size: 13px;
		color: #c8d6f0;
		margin-bottom: 8px;
	}

	.message-item header span,
	.message-item header em {
		color: #96a9c9;
		font-size: 12px;
	}

	.composer {
		border: 1px solid #2f3c58;
		border-radius: 10px;
		padding: 10px;
		background: #151c2b;
	}

	.edit-box {
		display: grid;
		gap: 8px;
	}

	.actions-row {
		display: flex;
		gap: 8px;
		margin-top: 8px;
		flex-wrap: wrap;
	}

	.checkbox-row {
		margin-top: 8px;
		display: flex;
		align-items: center;
		gap: 8px;
		color: #c8d6f0;
	}

	.checkbox-row input {
		width: auto;
		margin: 0;
	}

	button {
		padding: 8px 12px;
		border: 0;
		border-radius: 8px;
		background: #2f63ff;
		color: white;
		cursor: pointer;
	}

	button.ghost {
		background: #25304a;
	}

	button.danger {
		background: #8e2a3f;
	}

	button:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	.link-btn {
		background: transparent;
		color: #9fc2ff;
		padding: 0;
	}

	textarea,
	input {
		width: 100%;
		box-sizing: border-box;
		padding: 8px;
		margin-top: 6px;
		margin-bottom: 8px;
		background: #0f1521;
		border: 1px solid #2f3c58;
		border-radius: 8px;
		color: #e7eefc;
		font-family: inherit;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 13px;
	}

	th,
	td {
		padding: 8px;
		border-bottom: 1px solid #2f3c58;
		text-align: left;
		vertical-align: top;
	}

	th {
		color: #9fb1cf;
		font-size: 12px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	code.block {
		display: block;
		padding: 8px;
		border-radius: 8px;
		background: #0e1420;
		word-break: break-all;
	}

	.video-source-list {
		display: grid;
		gap: 8px;
		margin-bottom: 12px;
	}

	.video-source-item {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 10px;
		padding: 8px;
		border-radius: 8px;
		border: 1px solid #2f3c58;
		background: #0f1521;
	}

	.video-source-item div {
		display: grid;
		gap: 2px;
	}

	.video-source-item span {
		color: #9fb1cf;
		font-size: 13px;
	}

	.video-grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 10px;
	}

	:global(.remote-video) {
		width: 100%;
		aspect-ratio: 16 / 9;
		object-fit: cover;
		border-radius: 10px;
		border: 1px solid #2f3c58;
		background: #05080f;
	}

	.hidden-media-sink {
		display: none;
	}

	.markdown-body :global(p) {
		margin: 0 0 8px;
	}

	.markdown-body :global(p:last-child) {
		margin-bottom: 0;
	}

	.markdown-body :global(pre) {
		background: #0f1521;
		padding: 8px;
		border-radius: 8px;
		overflow: auto;
	}

	.ok {
		color: #7ef2ab;
	}

	.fail,
	.error {
		color: #ff7d7d;
	}

	.muted {
		color: #9fb1cf;
	}

	.used {
		color: #c3d2ee;
		opacity: 0.8;
	}

	@media (max-width: 980px) {
		.server-layout {
			grid-template-columns: 1fr;
			min-height: auto;
		}

		.channel-sidebar {
			border-right: 0;
			border-bottom: 1px solid #273149;
		}
	}
</style>
